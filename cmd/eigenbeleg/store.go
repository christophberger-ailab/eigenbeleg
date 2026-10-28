package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// record is an Eigenbeleg as stored by the web UI. Field values are kept as
// entered; defaults (subject, type) are applied only when generating the PDF.
type record struct {
	ID        int       `json:"id"`
	Number    string    `json:"number"`
	Amount    float64   `json:"amount"`
	Subject   string    `json:"subject,omitempty"`
	Recipient string    `json:"recipient"`
	Type      string    `json:"type,omitempty"`
	Reason    string    `json:"reason"`
	Date      string    `json:"date"` // YYYY-MM-DD
	Signature string    `json:"signature"`
	Header    string    `json:"header,omitempty"`
	Hidden    bool      `json:"hidden,omitempty"`
	Created   time.Time `json:"created"`
	Updated   time.Time `json:"updated"`
}

// store keeps all records in a single JSON file. Every change rewrites the
// file atomically.
type store struct {
	mu      sync.Mutex
	path    string
	NextID  int      `json:"next_id"`
	Records []record `json:"receipts"`
}

var errNotFound = errors.New("Eigenbeleg not found")

// openStore loads the store from path. A missing file yields an empty store;
// the file is created on the first change.
func openStore(path string) (*store, error) {
	s := &store{path: path, NextID: 1}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading store: %w", err)
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("parsing store %s: %w", path, err)
	}
	return s, nil
}

// list returns all records in chronological order: by date, then by
// creation order.
func (s *store) list() []record {
	s.mu.Lock()
	defer s.mu.Unlock()
	recs := slices.Clone(s.Records)
	slices.SortStableFunc(recs, func(a, b record) int {
		if c := strings.Compare(a.Date, b.Date); c != 0 {
			return c
		}
		return a.ID - b.ID
	})
	return recs
}

func (s *store) get(id int) (record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i := s.index(id); i >= 0 {
		return s.Records[i], nil
	}
	return record{}, errNotFound
}

// create assigns a new ID to r, stores it, and returns the stored record.
func (s *store) create(r record) (record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r.ID = s.NextID
	r.Created = time.Now()
	r.Updated = r.Created
	s.NextID++
	s.Records = append(s.Records, r)
	return r, s.save()
}

// update replaces the record with r.ID, keeping its creation time and
// hidden state.
func (s *store) update(r record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.index(r.ID)
	if i < 0 {
		return errNotFound
	}
	r.Created = s.Records[i].Created
	r.Hidden = s.Records[i].Hidden
	r.Updated = time.Now()
	s.Records[i] = r
	return s.save()
}

// setHidden hides or unhides a record. Hidden records stay in the store and
// can be restored.
func (s *store) setHidden(id int, hidden bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.index(id)
	if i < 0 {
		return errNotFound
	}
	s.Records[i].Hidden = hidden
	s.Records[i].Updated = time.Now()
	return s.save()
}

// nextNumber suggests a receipt number for a new record by incrementing the
// trailing digits of the most recently created record's number, keeping
// their width, e.g. 2026-007 -> 2026-008.
func (s *store) nextNumber() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.Records) == 0 {
		return ""
	}
	last := s.Records[len(s.Records)-1].Number
	end := len(last)
	start := end
	for start > 0 && last[start-1] >= '0' && last[start-1] <= '9' {
		start--
	}
	if start == end {
		return ""
	}
	n, err := strconv.Atoi(last[start:end])
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s%0*d", last[:start], end-start, n+1)
}

func (s *store) index(id int) int {
	return slices.IndexFunc(s.Records, func(r record) bool { return r.ID == id })
}

// save writes the store to a temporary file and renames it over the store
// file, so that a crash never leaves a truncated file behind. The caller
// must hold s.mu.
func (s *store) save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".eigenbelege-*.json")
	if err != nil {
		return fmt.Errorf("saving store: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("saving store: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("saving store: %w", err)
	}
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		return fmt.Errorf("saving store: %w", err)
	}
	return nil
}
