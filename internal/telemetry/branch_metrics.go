package telemetry

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"
)

// BranchProfile holds aggregated metrics for a single branch, enriched with
// classification and artifact linkage.
type BranchProfile struct {
	Branch          string   `json:"branch"`
	BranchType      string   `json:"branch_type"`
	Sessions        int      `json:"sessions"`
	TotalTokens     int      `json:"total_tokens"`
	AvgTokens       float64  `json:"avg_tokens_per_session"`
	TotalModelCalls int      `json:"total_model_calls"`
	TotalToolCalls  int      `json:"total_tool_calls"`
	AvgPeakUtil     *float64 `json:"avg_peak_utilization,omitempty"`
	TaskCount       int      `json:"task_count"`
	PRNumber        string   `json:"pr_number,omitempty"`
	ShipmentID      string   `json:"shipment_id,omitempty"`
	FeatureID       string   `json:"feature_id,omitempty"`
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
}

// AggregateBranches groups sessions by branch name and computes per-branch
// totals, averages, branch type classification, and artifact ID extraction.
// Ghost sessions are excluded. Results are sorted by LastSeen descending.
func AggregateBranches(sessions []SessionSummaryRecord) []BranchProfile {
	type branchAccum struct {
		profile      BranchProfile
		peakUtilSum  float64
		peakUtilCount int
	}

	index := make(map[string]*branchAccum)
	var order []string

	for _, s := range sessions {
		if IsGhostSession(s) {
			continue
		}
		key := s.Branch
		acc, ok := index[key]
		if !ok {
			shipID, featID := ExtractArtifactIDs(key)
			acc = &branchAccum{
				profile: BranchProfile{
					Branch:     key,
					BranchType: DeriveBranchType(key),
					ShipmentID: shipID,
					FeatureID:  featID,
					FirstSeen:  s.HarvestedAt,
					LastSeen:   s.HarvestedAt,
				},
			}
			index[key] = acc
			order = append(order, key)
		}
		p := &acc.profile
		p.Sessions++
		p.TotalTokens += s.TotalTokens
		p.TotalModelCalls += s.ModelCalls
		p.TotalToolCalls += s.ToolCalls
		p.TaskCount += len(s.CompletedTasks)

		if s.PeakUtilization != nil {
			acc.peakUtilSum += *s.PeakUtilization
			acc.peakUtilCount++
		}

		if s.HarvestedAt.Before(p.FirstSeen) {
			p.FirstSeen = s.HarvestedAt
		}
		if s.HarvestedAt.After(p.LastSeen) {
			p.LastSeen = s.HarvestedAt
		}
	}

	profiles := make([]BranchProfile, 0, len(order))
	for _, key := range order {
		acc := index[key]
		p := &acc.profile
		if p.Sessions > 0 {
			p.AvgTokens = float64(p.TotalTokens) / float64(p.Sessions)
		}
		if acc.peakUtilCount > 0 {
			avg := acc.peakUtilSum / float64(acc.peakUtilCount)
			p.AvgPeakUtil = &avg
		}
		profiles = append(profiles, *p)
	}

	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].LastSeen.After(profiles[j].LastSeen)
	})

	return profiles
}

// artifactIDPattern matches numeric prefixes in branch names for artifact ID extraction.
var artifactIDPattern = regexp.MustCompile(`^(\d+)`)

// ExtractArtifactIDs parses known branch naming patterns to extract backlogit
// shipment and feature IDs. Returns empty strings when the pattern does not match.
//
// Patterns:
//   - feature/{NNN}-f-* or feature/{NNN}-F-* → featureID = "{NNN}-F"
//   - ship/{NNN}s-* → shipmentID = "{NNN}-S"
//   - chore/stage-{NNN}-s-* or chore/stage-{NNN}s-* → shipmentID = "{NNN}-S"
func ExtractArtifactIDs(branch string) (shipmentID, featureID string) {
	switch {
	case strings.HasPrefix(branch, "feature/"):
		suffix := strings.TrimPrefix(branch, "feature/")
		if m := artifactIDPattern.FindString(suffix); m != "" {
			// Check for -f- or -F- after the number
			rest := suffix[len(m):]
			if strings.HasPrefix(rest, "-f-") || strings.HasPrefix(rest, "-F-") ||
				strings.HasPrefix(rest, "-f") || strings.HasPrefix(rest, "-F") {
				featureID = fmt.Sprintf("%s-F", m)
			}
		}
	case strings.HasPrefix(branch, "ship/"):
		suffix := strings.TrimPrefix(branch, "ship/")
		if m := artifactIDPattern.FindString(suffix); m != "" {
			rest := suffix[len(m):]
			if strings.HasPrefix(rest, "s-") || strings.HasPrefix(rest, "s") {
				shipmentID = fmt.Sprintf("%s-S", m)
			}
		}
	case strings.HasPrefix(branch, "chore/stage-"):
		suffix := strings.TrimPrefix(branch, "chore/stage-")
		if m := artifactIDPattern.FindString(suffix); m != "" {
			rest := suffix[len(m):]
			if strings.HasPrefix(rest, "-s-") || strings.HasPrefix(rest, "-s") ||
				strings.HasPrefix(rest, "s-") || strings.HasPrefix(rest, "s") {
				shipmentID = fmt.Sprintf("%s-S", m)
			}
		}
	}
	return shipmentID, featureID
}

// prMergePattern matches GitHub merge commit messages like:
// "abc1234 Merge pull request #109 from softwaresalt/feature/057-f-slug"
var prMergePattern = regexp.MustCompile(`Merge pull request #(\d+) from [^/]+/(.+)$`)

// ParseGitMergePRs runs git log to extract branch→PR number mappings from merge
// commits. Returns an empty map (not an error) when git is unavailable or the
// repo has no merge commits. Returns an error if parsing the git output fails
// due to I/O or buffer-overflow issues.
func ParseGitMergePRs(repoPath string) (map[string]string, error) {
	cmd := exec.Command("git", "-C", repoPath, "log", "--merges", "--oneline", "--all")
	output, err := cmd.Output()
	if err != nil {
		// Git not available or not a repo — graceful degradation.
		return map[string]string{}, nil
	}
	return ParseMergeLines(strings.NewReader(string(output)))
}

// ParseMergeLines parses git log --merges --oneline output and extracts
// branch name → PR number mappings. Returns an error if the scanner
// encounters an I/O or buffer-overflow issue.
func ParseMergeLines(r io.Reader) (map[string]string, error) {
	result := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		matches := prMergePattern.FindStringSubmatch(line)
		if len(matches) == 3 {
			prNum := "#" + matches[1]
			branch := matches[2]
			// Only record the first (most recent) PR for each branch.
			if _, exists := result[branch]; !exists {
				result[branch] = prNum
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("scan merge lines: %w", err)
	}
	return result, nil
}

// EnrichBranchProfiles fills PRNumber on each profile from the provided
// branch→PR mapping.
func EnrichBranchProfiles(profiles []BranchProfile, prMap map[string]string) {
	for i := range profiles {
		if pr, ok := prMap[profiles[i].Branch]; ok {
			profiles[i].PRNumber = pr
		}
	}
}
