package redhonesty

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

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
	if !faultline.ValidCommitSHA(baseSHA) || !faultline.ValidCommitSHA(headSHA) {
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
// directory as <root>/<task>/<commit>.json. Every filesystem operation after the
// root is opened goes through os.Root, so a symlink inside the store can never
// redirect a read or a write outside it, and recorded evidence is created with
// O_EXCL so it cannot be silently replaced.
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

func recordName(task, commit string) (string, error) {
	if !taskIDPattern.MatchString(task) {
		return "", fmt.Errorf("redhonesty: invalid task id %q", truncate(task))
	}
	if !faultline.ValidCommitSHA(commit) {
		return "", fmt.Errorf("redhonesty: invalid commit %q", truncate(commit))
	}
	return commit + ".json", nil
}

func isRedGreen(a faultline.EvidenceArtifact) bool {
	return a.NodeFamily == faultline.NodeFamilyRedGreen && a.SchemaVersion == faultline.EvidenceSchemaVersionV2
}

// taskRoot opens <store>/<task> as an os.Root. The store root and the task
// directory must be real directories (no symlink or junction); with create set
// both are created as needed.
func (s *Store) taskRoot(task string, create bool) (*os.Root, error) {
	if create {
		if err := os.MkdirAll(s.dir, 0o755); err != nil {
			return nil, err
		}
	}
	fi, err := os.Lstat(s.dir)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, errors.New("redhonesty: evidence store root is not a real directory")
	}
	root, err := os.OpenRoot(s.dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	if create {
		if err := root.Mkdir(task, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	di, err := root.Lstat(task)
	if err != nil {
		return nil, err
	}
	if !di.IsDir() {
		return nil, fmt.Errorf("redhonesty: evidence directory for %s is not a real directory", task)
	}
	return root.OpenRoot(task)
}

// Write validates a through the shared contract (a full decode round trip) and
// persists it without ever replacing an existing record. Re-recording the same
// canonical artifact is idempotent; a different artifact for the same task and
// commit is refused. Only red_green artifacts are accepted.
func (s *Store) Write(a faultline.EvidenceArtifact) (string, error) {
	if !isRedGreen(a) {
		return "", fmt.Errorf("redhonesty: store accepts only red_green v%d artifacts", faultline.EvidenceSchemaVersionV2)
	}
	name, err := recordName(a.ProducingTask, a.ProducingCommit)
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
	tr, err := s.taskRoot(a.ProducingTask, true)
	if err != nil {
		return "", err
	}
	defer tr.Close()
	path := filepath.Join(s.dir, a.ProducingTask, name)

	f, err := tr.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err == nil {
		if err := writeSynced(f, append(raw, '\n')); err != nil {
			_ = tr.Remove(name) // never leave a partial record behind
			return "", fmt.Errorf("redhonesty: write evidence: %w", err)
		}
		return path, nil
	}
	if !errors.Is(err, os.ErrExist) {
		return "", fmt.Errorf("redhonesty: create evidence: %w", err)
	}
	existing, lerr := loadFrom(tr, a.ProducingTask, a.ProducingCommit, name, true)
	if lerr != nil {
		return "", fmt.Errorf("redhonesty: existing evidence for %s at %s is unreadable or invalid: %w", a.ProducingTask, a.ProducingCommit[:12], lerr)
	}
	old, e1 := existing.Canonical()
	cur, e2 := a.Canonical()
	if e1 == nil && e2 == nil && bytes.Equal(old, cur) {
		return path, nil
	}
	return "", fmt.Errorf("redhonesty: evidence already recorded for %s at %s and differs", a.ProducingTask, a.ProducingCommit[:12])
}

func writeSynced(f *os.File, data []byte) error {
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// Load retrieves and re-validates the artifact for a task and commit. The file
// must be a regular file within the size budget and must be bound to exactly
// the requested task and commit, so an artifact copied to another key is
// refused. Special files (for example a FIFO) are never opened blocking.
func (s *Store) Load(task, commit string) (faultline.EvidenceArtifact, error) {
	var zero faultline.EvidenceArtifact
	name, err := recordName(task, commit)
	if err != nil {
		return zero, err
	}
	tr, err := s.taskRoot(task, false)
	if err != nil {
		return zero, err
	}
	defer tr.Close()
	return loadFrom(tr, task, commit, name, false)
}

// loadFrom reads one record from an already-opened task root. When retry is set
// a decode failure is retried briefly, covering a concurrent writer that created
// the file but has not finished writing it.
func loadFrom(tr *os.Root, task, commit, name string, retry bool) (faultline.EvidenceArtifact, error) {
	var zero faultline.EvidenceArtifact
	attempts := 1
	if retry {
		attempts = 10
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		a, err := readRecord(tr, name)
		if err == nil {
			if !isRedGreen(a) || a.ProducingTask != task || a.ProducingCommit != commit {
				return zero, errors.New("redhonesty: stored evidence is not bound to the requested task and commit")
			}
			return a, nil
		}
		lastErr = err
		if !retry || errors.Is(err, os.ErrNotExist) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	return zero, lastErr
}

func readRecord(tr *os.Root, name string) (faultline.EvidenceArtifact, error) {
	var zero faultline.EvidenceArtifact
	// Check the entry type BEFORE opening so a FIFO or device cannot block the
	// open, then open non-blocking (where supported) and re-validate the
	// descriptor so a swap between the two cannot recreate the hang.
	li, err := tr.Lstat(name)
	if err != nil {
		return zero, err
	}
	if !li.Mode().IsRegular() {
		return zero, fmt.Errorf("redhonesty: evidence %s is not a regular file", name)
	}
	f, err := tr.OpenFile(name, os.O_RDONLY|readOnlyOpenFlags, 0)
	if err != nil {
		return zero, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return zero, err
	}
	if !fi.Mode().IsRegular() {
		return zero, fmt.Errorf("redhonesty: evidence %s is not a regular file", name)
	}
	raw, err := io.ReadAll(io.LimitReader(f, faultline.MaxArtifactBytes+2))
	if err != nil {
		return zero, err
	}
	// Write appends exactly one newline; strip it before the size check so an
	// artifact of exactly MaxArtifactBytes round-trips.
	raw = bytes.TrimSuffix(raw, []byte{'\n'})
	if len(raw) > faultline.MaxArtifactBytes {
		return zero, fmt.Errorf("redhonesty: evidence exceeds %d bytes", faultline.MaxArtifactBytes)
	}
	a, _, err := faultline.DecodeAndValidate(raw)
	if err != nil {
		return zero, fmt.Errorf("redhonesty: stored evidence invalid: %w", err)
	}
	return a, nil
}
