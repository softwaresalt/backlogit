package redhonesty

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"

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
// directory as <root>/<task>/<commit>.json. Reads, staging and directory
// creation go through os.Root, so a symlink inside the store cannot redirect
// them outside it. Records are published atomically with a no-clobber hard link
// of a fully written staging file (the final name never exists half-written and
// is never replaced). The link step itself is path-based because os.Root has no
// Link before Go 1.25; it runs on a task directory re-resolved and re-verified
// inside the store immediately before, and an attacker able to swap store
// directories already holds the privilege to write evidence directly, which is
// why consumers authenticate evidence rather than trusting its location.
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

	// Stage the complete record in a temp file created through the Root (so it
	// cannot escape the store), sync it, then publish it with a no-clobber hard
	// link. The final name therefore never exists half-written: an interruption
	// leaves at most a stray staging file, never a torn record that would block
	// retries. A concurrent publisher that loses sees the winner's complete record.
	tmpName, err := stageRecord(tr, append(raw, '\n'))
	if err != nil {
		return "", fmt.Errorf("redhonesty: stage evidence: %w", err)
	}
	defer tr.Remove(tmpName)
	realDir, err := s.realTaskDir(a.ProducingTask)
	if err != nil {
		return "", err
	}
	err = os.Link(filepath.Join(realDir, tmpName), filepath.Join(realDir, name))
	if err == nil {
		return path, nil
	}
	if !errors.Is(err, os.ErrExist) {
		return "", fmt.Errorf("redhonesty: publish evidence: %w", err)
	}
	existing, lerr := loadFrom(tr, a.ProducingTask, a.ProducingCommit, name)
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

// realTaskDir resolves <store>/<task> and proves it still lies inside the
// resolved store root immediately before the path-based publish step.
func (s *Store) realTaskDir(task string) (string, error) {
	realRoot, err := filepath.EvalSymlinks(s.dir)
	if err != nil {
		return "", err
	}
	realDir, err := filepath.EvalSymlinks(filepath.Join(s.dir, task))
	if err != nil {
		return "", err
	}
	if !within(realRoot, realDir) || realDir == realRoot {
		return "", fmt.Errorf("redhonesty: evidence directory for %s escapes the store", task)
	}
	return realDir, nil
}

// stageRecord writes data to a new, synced, uniquely named file in tr and
// returns its name.
func stageRecord(tr *os.Root, data []byte) (string, error) {
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	name := ".evidence-" + hex.EncodeToString(nonce[:]) + ".tmp"
	f, err := tr.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	if err := writeSynced(f, data); err != nil {
		_ = tr.Remove(name)
		return "", err
	}
	return name, nil
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
	return loadFrom(tr, task, commit, name)
}

// loadFrom reads one record from an already-opened task root and checks it is
// bound to exactly the requested task and commit. Records are published
// atomically, so a record is never observed half-written.
func loadFrom(tr *os.Root, task, commit, name string) (faultline.EvidenceArtifact, error) {
	var zero faultline.EvidenceArtifact
	a, err := readRecord(tr, name)
	if err != nil {
		return zero, err
	}
	if !isRedGreen(a) || a.ProducingTask != task || a.ProducingCommit != commit {
		return zero, errors.New("redhonesty: stored evidence is not bound to the requested task and commit")
	}
	return a, nil
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
