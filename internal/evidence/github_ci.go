package evidence

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// CheckState is the normalized conclusion of one CI check.
type CheckState string

const (
	// CheckPassed is a successful, neutral or skipped check.
	CheckPassed CheckState = "passed"
	// CheckFailed is a failed, cancelled, errored or timed-out check.
	CheckFailed CheckState = "failed"
	// CheckPending is a check that has not completed.
	CheckPending CheckState = "pending"
)

// Check is one CI check on the PR head.
type Check struct {
	Name  string
	State CheckState
}

// CIResult is the derived CI and merge evidence for one PR.
type CIResult struct {
	Status  Status
	Reason  string
	PR      int
	HeadSHA string
	Checks  []Check
	// AllPassed is true only when at least one check exists and every check passed.
	AllPassed bool
	Merged    bool
	MergeSHA  string
}

type ciPayload struct {
	HeadRefOid  string `json:"headRefOid"`
	State       string `json:"state"`
	MergeCommit *struct {
		Oid string `json:"oid"`
	} `json:"mergeCommit"`
	StatusCheckRollup []struct {
		Typename   string `json:"__typename"`
		Name       string `json:"name"`
		Context    string `json:"context"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		State      string `json:"state"`
	} `json:"statusCheckRollup"`
}

func normalizeCheckRun(status, conclusion string) CheckState {
	if !strings.EqualFold(status, "COMPLETED") {
		return CheckPending
	}
	switch strings.ToUpper(conclusion) {
	case "SUCCESS", "NEUTRAL", "SKIPPED":
		return CheckPassed
	default:
		return CheckFailed
	}
}

func normalizeStatusContext(state string) CheckState {
	switch strings.ToUpper(state) {
	case "SUCCESS":
		return CheckPassed
	case "PENDING", "EXPECTED":
		return CheckPending
	default:
		return CheckFailed
	}
}

// CIEvidence derives CI check outcomes and the merge SHA for a PR. Any failure
// yields StatusDegraded with no partial evidence.
func (g *GitHub) CIEvidence(ctx context.Context, pr int) CIResult {
	fail := func(reason string) CIResult {
		return CIResult{Status: StatusDegraded, Reason: reason, PR: pr}
	}
	if _, _, ok := splitRepo(g.Repo); !ok {
		return fail(fmt.Sprintf("invalid repository slug %q", g.Repo))
	}
	out, reason := g.ghRun(ctx, "gh pr view",
		"pr", "view", strconv.Itoa(pr), "--repo", g.Repo,
		"--json", "headRefOid,state,mergeCommit,statusCheckRollup")
	if reason != "" {
		return fail(reason)
	}
	var p ciPayload
	if err := json.Unmarshal(out, &p); err != nil {
		return fail(degraded("decode pr payload", err, nil, g.Token))
	}
	if p.HeadRefOid == "" {
		return fail(fmt.Sprintf("pull request %d not found", pr))
	}

	res := CIResult{Status: StatusVerified, PR: pr, HeadSHA: p.HeadRefOid}
	allPassed := len(p.StatusCheckRollup) > 0
	for _, c := range p.StatusCheckRollup {
		chk := Check{Name: c.Name}
		if c.Typename == "StatusContext" {
			chk.Name = c.Context
			chk.State = normalizeStatusContext(c.State)
		} else {
			chk.State = normalizeCheckRun(c.Status, c.Conclusion)
		}
		if chk.State != CheckPassed {
			allPassed = false
		}
		res.Checks = append(res.Checks, chk)
	}
	res.AllPassed = allPassed

	if strings.EqualFold(p.State, "MERGED") {
		if p.MergeCommit == nil || p.MergeCommit.Oid == "" {
			return fail(fmt.Sprintf("pull request %d is merged but has no merge commit", pr))
		}
		res.Merged = true
		res.MergeSHA = p.MergeCommit.Oid
	}
	return res
}
