package stash

import (
	"io"
)

// WriteJSONL serializes a slice of stash entries to the given writer in JSONL format.
// Each entry is written as a single JSON object per line.
//
// Worker: Marshal each Entry to JSON and write one line per entry. Use compact
// JSON (no indentation). Entries must preserve ID, Priority, Kind, Text, and
// DeliberationID fields. Return an error if marshaling or writing fails.
func WriteJSONL(w io.Writer, entries []Entry) error {
	panic("not implemented: Worker: Serialize entries to JSONL with one JSON object per line")
}

// ReadJSONL deserializes stash entries from a JSONL reader.
// Each line must be a valid JSON object representing one Entry.
//
// Worker: Read the input line by line, unmarshal each line into an Entry struct,
// and return the collected entries. Skip empty lines. Return an error if any line
// fails to unmarshal.
func ReadJSONL(r io.Reader) ([]Entry, error) {
	panic("not implemented: Worker: Read JSONL line by line, unmarshal each to Entry, skip blanks")
}

// MigrateStashMDToJSONL reads the legacy .stash.md file at srcPath, parses entries,
// and writes them as JSONL to dstPath using atomic temp-file-then-rename.
//
// Worker: Parse srcPath using ParseFile, convert entries to JSONL via WriteJSONL
// into a temp file, then atomically rename to dstPath. Log entry count with slog.Info.
// Return the number of migrated entries and any error.
func MigrateStashMDToJSONL(srcPath, dstPath string) (int, error) {
	panic("not implemented: Worker: Parse .stash.md, write JSONL to temp file, atomic rename to dstPath, log counts")
}
