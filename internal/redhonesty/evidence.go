package redhonesty

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
	"github.com/softwaresalt/backlogit/internal/faultline"
)

// ValidatorIdentity is the producer identity recorded in every artifact.
const ValidatorIdentity = "backlogit/redhonesty@v1"

// ErrNoNewTests is returned when a change adds no regression tests: there is
// nothing to verify and no evidence is produced.
var ErrNoNewTests = errors.New("redhonesty: no newly added tests")

var taskIDPattern = regexp.MustCompile(`^\d{3,}(\.\d{3,})*-[A-Z]{1,2}$`)

// BuildArtifact converts verdicts into a RedGreenEvidence EvidenceArtifact
// conforming to the shared faultline contract. The artifact is bound to the
// producing task and head commit and is rejected unless it validates against
// the shared schema. Status is pass only when every test is honest.
func BuildArtifact(taskID, baseSHA, headSHA string, verdicts []Verdict) (faultline.EvidenceArtifact, error) {
	var zero faultline.EvidenceArtifact
	if !taskIDPattern.MatchString(taskID) {
		return zero, fmt.Errorf("redhonesty: invalid task id %q", truncate(taskID))
	}
	if !shaPattern.MatchString(baseSHA) || !shaPattern.MatchString(headSHA) {
		return zero, errors.New("redhonesty: base and head must be full commit SHAs")
	}
	if len(verdicts) == 0 {
		return zero, ErrNoNewTests
	}
	status := faultline.StatusPass
	tests := make([]faultline.RedGreenTest, 0, len(verdicts))
	for _, v := range verdicts {
		classes := make([]string, 0, len(v.Rejections))
		for _, r := range v.Rejections {
			classes = append(classes, string(r))
		}
		sort.Strings(classes)
		if !v.Honest() {
			status = faultline.StatusFail
		}
		tests = append(tests, faultline.RedGreenTest{
			Package:          v.Test.Package,
			Test:             v.Test.Name,
			BaselineResult:   string(v.Baseline),
			HeadResult:       string(v.Head),
			Honest:           v.Honest(),
			RejectionClasses: classes,
		})
	}
	sort.Slice(tests, func(i, j int) bool {
		if tests[i].Package != tests[j].Package {
			return tests[i].Package < tests[j].Package
		}
		return tests[i].Test < tests[j].Test
	})
	raw, err := json.Marshal(faultline.RedGreenEvidence{BaseSHA: baseSHA, HeadSHA: headSHA, Tests: tests})
	if err != nil {
		return zero, err
	}
	a := faultline.EvidenceArtifact{
		SchemaVersion:     faultline.EvidenceSchemaVersionV2,
		ProducingTask:     taskID,
		ProducingCommit:   headSHA,
		NodeFamily:        faultline.NodeFamilyRedGreen,
		Applicability:     []string{"internal"},
		ValidatorIdentity: ValidatorIdentity,
		VerifiedEvidence:  raw,
		Status:            status,
	}
	if err := faultline.Validate(a); err != nil {
		return zero, fmt.Errorf("redhonesty: artifact violates the shared evidence contract: %w", err)
	}
	return a, nil
}

// Store persists RedGreenEvidence artifacts per task and commit under a root
// directory as <root>/<task>/<commit>.json.
type Store struct{ dir string }

// NewStore creates a Store rooted at dir. The directory is created lazily.
func NewStore(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("redhonesty: store directory is required")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	return &Store{dir: abs}, nil
}

func (s *Store) pathFor(task, commit string) (string, error) {
	if !taskIDPattern.MatchString(task) {
		return "", fmt.Errorf("redhonesty: invalid task id %q", truncate(task))
	}
	if !shaPattern.MatchString(commit) {
		return "", fmt.Errorf("redhonesty: invalid commit %q", truncate(commit))
	}
	return filepath.Join(s.dir, task, commit+".json"), nil
}

func isRedGreen(a faultline.EvidenceArtifact) bool {
	return a.NodeFamily == faultline.NodeFamilyRedGreen && a.SchemaVersion == faultline.EvidenceSchemaVersionV2
}

// Write validates a through the shared contract (a full decode round trip) and
// persists it atomically. Only red_green artifacts are accepted.
func (s *Store) Write(a faultline.EvidenceArtifact) (string, error) {
	if !isRedGreen(a) {
		return "", fmt.Errorf("redhonesty: store accepts only red_green v%d artifacts", faultline.EvidenceSchemaVersionV2)
	}
	p, err := s.pathFor(a.ProducingTask, a.ProducingCommit)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	if _, _, err := faultline.DecodeAndValidate(raw); err != nil {
		return "", fmt.Errorf("redhonesty: refusing to persist invalid evidence: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	if err := atomicfile.WriteFileAtomic(p, append(raw, '\n')); err != nil {
		return "", err
	}
	return p, nil
}

// Load retrieves and re-validates the artifact for a task and commit. The file
// must be a regular file within the size budget and must be bound to exactly
// the requested task and commit, so an artifact copied to another key is
// refused.
func (s *Store) Load(task, commit string) (faultline.EvidenceArtifact, error) {
	var zero faultline.EvidenceArtifact
	p, err := s.pathFor(task, commit)
	if err != nil {
		return zero, err
	}
	fi, err := os.Lstat(p)
	if err != nil {
		return zero, err
	}
	if !fi.Mode().IsRegular() {
		return zero, fmt.Errorf("redhonesty: evidence %s is not a regular file", filepath.Base(p))
	}
	f, err := os.Open(p)
	if err != nil {
		return zero, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, faultline.MaxArtifactBytes+2))
	if err != nil {
		return zero, err
	}
	if len(raw) > faultline.MaxArtifactBytes+1 { // allow the trailing newline
		return zero, fmt.Errorf("redhonesty: evidence exceeds %d bytes", faultline.MaxArtifactBytes)
	}
	a, _, err := faultline.DecodeAndValidate(raw)
	if err != nil {
		return zero, fmt.Errorf("redhonesty: stored evidence invalid: %w", err)
	}
	if !isRedGreen(a) || a.ProducingTask != task || a.ProducingCommit != commit {
		return zero, errors.New("redhonesty: stored evidence is not bound to the requested task and commit")
	}
	return a, nil
}
