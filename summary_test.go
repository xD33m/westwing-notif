package main

import (
	"math/rand"
	"strings"
	"testing"
	"time"
)

func TestParseSchedule(t *testing.T) {
	s, err := ParseSchedule("Sun 10:00")
	if err != nil || s.Off || s.Day != time.Sunday || s.Hour != 10 || s.Minute != 0 {
		t.Errorf("Sun 10:00 → %+v, %v", s, err)
	}
	s, err = ParseSchedule("off")
	if err != nil || !s.Off {
		t.Errorf("off → %+v, %v", s, err)
	}
	if _, err := ParseSchedule("Funday 1:00"); err == nil {
		t.Error("want error for Funday")
	}
}

func TestScheduleNext(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	s := Schedule{Day: time.Sunday, Hour: 10}
	cases := []struct{ from, want time.Time }{
		{time.Date(2026, 10, 17, 12, 0, 0, 0, berlin), time.Date(2026, 10, 18, 10, 0, 0, 0, berlin)},
		{time.Date(2026, 10, 18, 10, 0, 0, 0, berlin), time.Date(2026, 10, 25, 10, 0, 0, 0, berlin)},
		{time.Date(2026, 10, 24, 23, 0, 0, 0, berlin), time.Date(2026, 10, 25, 10, 0, 0, 0, berlin)}, // DST ends 2026-10-25 03:00
	}
	for _, c := range cases {
		got := s.Next(c.from)
		if !got.Equal(c.want) || got.Hour() != 10 {
			t.Errorf("Next(%v) = %v, want %v", c.from, got, c.want)
		}
	}
}

func TestBuildSummary(t *testing.T) {
	st := &State{Items: map[string]*ItemState{
		"https://a.de/1": {Name: "Libby", Price: 1049, Lowest: 999, InStock: true, LastSuccess: t0},
		"https://a.de/2": {Name: "Sofa", Price: 1049, Lowest: 1049, InStock: false, LastSuccess: t0},
	}}
	n := BuildSummary([]string{"https://a.de/1", "https://a.de/2", "https://a.de/never-seen"}, st)
	lines := strings.Split(n.Message, "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %q", n.Message)
	}
	if !strings.Contains(lines[0], "Libby") || !strings.Contains(lines[0], "1.049,00 €") || !strings.Contains(lines[0], "999,00 €") {
		t.Errorf("line 0 = %q", lines[0])
	}
	if !strings.Contains(lines[1], "ausverkauft") {
		t.Errorf("line 1 = %q", lines[1])
	}
	if n.Priority != 2 || n.Tags[0] != "bar_chart" {
		t.Errorf("priority/tags %d %v", n.Priority, n.Tags)
	}
}

func TestJitter(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 1000; i++ {
		d := Jitter(2*time.Hour, r)
		if d < 102*time.Minute || d > 138*time.Minute {
			t.Fatalf("Jitter = %v, outside ±15 %%", d)
		}
	}
}
