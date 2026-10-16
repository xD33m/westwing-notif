package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"
)

// ReadItems returns the product URLs in the items file: one per line,
// blank lines and # comments skipped, duplicates dropped.
func ReadItems(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var items []string
	seen := map[string]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || seen[line] {
			continue
		}
		seen[line] = true
		items = append(items, line)
	}
	return items, nil
}

// ItemState is everything remembered about one watched product.
type ItemState struct {
	Name            string    `json:"name"`
	Price           float64   `json:"price"`
	Currency        string    `json:"currency"`
	InStock         bool      `json:"in_stock"`
	Lowest          float64   `json:"lowest"`
	LowestAt        time.Time `json:"lowest_at"`
	LastSuccess     time.Time `json:"last_success"`
	Failures        int       `json:"failures"`
	FailureNotified bool      `json:"failure_notified"`
	LastError       string    `json:"last_error,omitempty"`
}

// State is the whole persisted state, keyed by product URL.
type State struct {
	Items       map[string]*ItemState `json:"items"`
	LastSummary time.Time             `json:"last_summary"`
}

// LoadState reads the state file. A missing file is an empty state.
func LoadState(path string) (*State, error) {
	st := &State{}
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if err == nil {
		if err := json.Unmarshal(b, st); err != nil {
			return nil, err
		}
	}
	if st.Items == nil {
		st.Items = map[string]*ItemState{}
	}
	return st, nil
}

// Save writes the state atomically (temp file + rename), so a crash
// mid-write never leaves a half-written state file behind.
func (s *State) Save(path string) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// AppendHistory adds one row per successful check to the CSV log.
func AppendHistory(path string, t time.Time, url string, p Product) error {
	_, statErr := os.Stat(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if errors.Is(statErr, fs.ErrNotExist) {
		w.Write([]string{"time", "url", "name", "price", "currency", "in_stock"})
	}
	w.Write([]string{
		t.Format(time.RFC3339), url, p.Name,
		strconv.FormatFloat(p.Price, 'f', 2, 64), p.Currency, strconv.FormatBool(p.InStock),
	})
	w.Flush()
	return w.Error()
}
