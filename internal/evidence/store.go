// Package evidence implements the append-only Historical Evidence Store
// (architecture.md section 5.4, ADR-005). The Event/Data Service is the
// only writer (docs/data-contracts.md section 6).
package evidence

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Record is the evidence envelope written to the JSONL file
// (docs/data-contracts.md section 6).
type Record struct {
	RecordType string          `json:"record_type"`
	RecordedAt string          `json:"recorded_at"`
	Payload    json.RawMessage `json:"payload"`
}

// Store is an append-only, file-backed evidence log. It survives process
// restart (REQ-DATA-002) because it is a plain file, not in-memory state.
type Store struct {
	mu   sync.Mutex
	file *os.File
}

// Open opens (creating if necessary) the evidence file at path for
// appending.
func Open(path string) (*Store, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("evidence: open %s: %w", path, err)
	}
	return &Store{file: f}, nil
}

// Close closes the underlying file.
func (s *Store) Close() error {
	return s.file.Close()
}

// Append writes one evidence record. payload is marshaled as-is.
func (s *Store) Append(recordType string, payload any) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("evidence: marshal payload: %w", err)
	}
	record := Record{
		RecordType: recordType,
		RecordedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Payload:    payloadBytes,
	}
	line, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("evidence: marshal record: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.file.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("evidence: append: %w", err)
	}
	return nil
}

// ReadAll reads every record currently in the file at path, in order. It
// is used by acceptance tests to assert end-to-end traceability
// (REQ-TEST-002); production components only ever append.
func ReadAll(path string) ([]Record, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("evidence: open %s: %w", path, err)
	}
	defer f.Close()

	var records []Record
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec Record
		if err := json.Unmarshal(line, &rec); err != nil {
			return nil, fmt.Errorf("evidence: parse line: %w", err)
		}
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("evidence: scan %s: %w", path, err)
	}
	return records, nil
}
