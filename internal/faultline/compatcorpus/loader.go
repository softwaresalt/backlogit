package compatcorpus

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
)

//go:embed testdata/corpus/manifest.json testdata/corpus/*/*.input testdata/corpus/*/*.normalized
var embeddedCorpus embed.FS

type manifestEntry struct {
	ID                 string      `json:"id"`
	Category           Category    `json:"category"`
	Adapter            string      `json:"adapter"`
	Expect             Expectation `json:"expect"`
	WantErr            string      `json:"want_err"`
	WantNormalizedFile string      `json:"want_normalized_file"`
}

func loadDefaultCorpus() ([]Entry, error) {
	corpusFS, err := fs.Sub(embeddedCorpus, "testdata/corpus")
	if err != nil {
		return nil, fmt.Errorf("open embedded corpus root: %w", err)
	}
	entries, err := loadCorpus(corpusFS)
	if err != nil {
		return nil, fmt.Errorf("read embedded corpus: %w", err)
	}
	return entries, nil
}

func loadCorpus(fsys fs.FS) ([]Entry, error) {
	if fsys == nil {
		return nil, fmt.Errorf("read manifest: %w", fs.ErrInvalid)
	}

	manifestData, err := fs.ReadFile(fsys, "manifest.json")
	if err != nil {
		return nil, fmt.Errorf("read manifest.json: %w", err)
	}

	decoder := json.NewDecoder(strings.NewReader(string(manifestData)))
	decoder.DisallowUnknownFields()
	var records []manifestEntry
	if err := decoder.Decode(&records); err != nil {
		return nil, fmt.Errorf("decode manifest.json: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("unexpected trailing JSON value")
		}
		return nil, fmt.Errorf("decode manifest.json trailing content: %w", err)
	}

	entries := make([]Entry, 0, len(records))
	seenIDs := make(map[string]struct{}, len(records))
	for index, record := range records {
		entry, err := loadManifestEntry(fsys, record)
		if err != nil {
			return nil, fmt.Errorf("manifest entry %d (%q): %w", index, record.ID, err)
		}
		if _, exists := seenIDs[entry.ID]; exists {
			return nil, fmt.Errorf("manifest entry %q: duplicate id: %w", entry.ID, fs.ErrInvalid)
		}
		seenIDs[entry.ID] = struct{}{}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].ID != entries[j].ID {
			return entries[i].ID < entries[j].ID
		}
		return entries[i].Adapter < entries[j].Adapter
	})
	return entries, nil
}

func loadManifestEntry(fsys fs.FS, record manifestEntry) (Entry, error) {
	if record.ID == "" ||
		!fs.ValidPath(record.ID) ||
		path.Base(record.ID) != record.ID ||
		strings.Contains(record.ID, `\`) {
		return Entry{}, fmt.Errorf("invalid id %q: %w", record.ID, fs.ErrInvalid)
	}
	if !validCategory(record.Category) {
		return Entry{}, fmt.Errorf("unknown category %q: %w", record.Category, fs.ErrInvalid)
	}
	if !validAdapterName(record.Adapter) {
		return Entry{}, fmt.Errorf("unknown adapter %q: %w", record.Adapter, fs.ErrInvalid)
	}

	entry := Entry{
		ID:       record.ID,
		Category: record.Category,
		Adapter:  record.Adapter,
		Expect:   record.Expect,
	}
	if err := validateManifestExpectation(record, &entry); err != nil {
		return Entry{}, err
	}

	inputPath := path.Join(string(record.Category), record.ID+".input")
	input, err := fs.ReadFile(fsys, inputPath)
	if err != nil {
		return Entry{}, fmt.Errorf("read input %q: %w", inputPath, err)
	}
	entry.Input = input

	if record.WantNormalizedFile != "" {
		if !fs.ValidPath(record.WantNormalizedFile) {
			return Entry{}, fmt.Errorf(
				"invalid normalized file %q: %w",
				record.WantNormalizedFile,
				fs.ErrInvalid,
			)
		}
		normalized, err := fs.ReadFile(fsys, record.WantNormalizedFile)
		if err != nil {
			return Entry{}, fmt.Errorf(
				"read normalized file %q: %w",
				record.WantNormalizedFile,
				err,
			)
		}
		entry.WantNormalized = normalized
	}
	return entry, nil
}

func validateManifestExpectation(record manifestEntry, entry *Entry) error {
	switch record.Expect {
	case ExpectRejected:
		if record.WantNormalizedFile != "" {
			return fmt.Errorf("rejected entry has normalized output: %w", fs.ErrInvalid)
		}
		if record.WantErr != "" {
			wantErr, ok := sentinelByName(record.WantErr)
			if !ok {
				return fmt.Errorf("unknown sentinel %q: %w", record.WantErr, fs.ErrInvalid)
			}
			entry.WantErr = wantErr
		}
	case ExpectAccepted:
		if record.WantErr != "" || record.WantNormalizedFile != "" {
			return fmt.Errorf("accepted entry has rejection or normalization fields: %w", fs.ErrInvalid)
		}
	case ExpectNormalized:
		if record.WantErr != "" || record.WantNormalizedFile == "" {
			return fmt.Errorf("normalized entry has invalid expected fields: %w", fs.ErrInvalid)
		}
	default:
		return fmt.Errorf("unknown expectation %q: %w", record.Expect, fs.ErrInvalid)
	}
	return nil
}

func sentinelByName(name string) (error, bool) {
	switch name {
	case "ErrTruncated":
		return ErrTruncated, true
	case "ErrMalformed":
		return ErrMalformed, true
	case "ErrDuplicateKey":
		return ErrDuplicateKey, true
	case "ErrCaseFoldCollision":
		return ErrCaseFoldCollision, true
	case "ErrOldVersion":
		return ErrOldVersion, true
	case "ErrTokenTooLong":
		return ErrTokenTooLong, true
	case "ErrInvalidPath":
		return ErrInvalidPath, true
	case "ErrUnclosedFront":
		return ErrUnclosedFront, true
	default:
		return nil, false
	}
}

func validCategory(category Category) bool {
	switch category {
	case CatMalformedJSON,
		CatTruncatedJSON,
		CatMalformedYAML,
		CatTruncatedYAML,
		CatDuplicateKey,
		CatCaseFoldedKey,
		CatCRLF,
		CatLF,
		CatOversizedToken,
		CatOldIndexVersion,
		CatWindowsSemantics:
		return true
	default:
		return false
	}
}

func validAdapterName(name string) bool {
	switch name {
	case "frontmatter", "events_jsonl", "scanner":
		return true
	default:
		return false
	}
}
