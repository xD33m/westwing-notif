package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestReadItems(t *testing.T) {
	path := filepath.Join(t.TempDir(), "items.txt")
	content := "# c\r\n\r\n  https://a.de/x.html  \r\nhttps://a.de/x.html\nhttps://b.de/y.html\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadItems(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"https://a.de/x.html", "https://b.de/y.html"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadItems = %q, want %q", got, want)
	}
}

func TestStateRoundTripAndMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	st, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState on missing file: %v", err)
	}
	if st.Items == nil || len(st.Items) != 0 {
		t.Fatalf("want empty non-nil map, got %#v", st.Items)
	}

	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	st.Items["https://a.de/x.html"] = &ItemState{Name: "Libby", Price: 1049, Currency: "EUR", InStock: true, Lowest: 1049, LowestAt: now, LastSuccess: now, Failures: 1, LastError: "boom"}
	st.LastSummary = now
	if err := st.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, st) {
		t.Errorf("round trip mismatch:\n got %#v\nwant %#v", got.Items["https://a.de/x.html"], st.Items["https://a.de/x.html"])
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temp file left behind")
	}
}

func TestAppendHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.csv")
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	p := Product{Name: "Libby, Holz", Price: 1049, Currency: "EUR", InStock: true}
	for i := 0; i < 2; i++ {
		if err := AppendHistory(path, now, "https://a.de/x.html", p); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d: %q", len(lines), b)
	}
	if lines[0] != "time,url,name,price,currency,in_stock" {
		t.Errorf("header = %q", lines[0])
	}
	if lines[1] != `2026-10-16T12:00:00Z,https://a.de/x.html,"Libby, Holz",1049.00,EUR,true` {
		t.Errorf("row = %q", lines[1])
	}
}
