package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// Schedule is a weekly point in time, e.g. "Sun 10:00".
type Schedule struct {
	Day          time.Weekday
	Hour, Minute int
	Off          bool
}

var weekdays = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

// ParseSchedule reads "Sun 10:00" or "off".
func ParseSchedule(s string) (Schedule, error) {
	s = strings.TrimSpace(s)
	if strings.EqualFold(s, "off") {
		return Schedule{Off: true}, nil
	}
	var day string
	var sch Schedule
	if _, err := fmt.Sscanf(s, "%s %d:%d", &day, &sch.Hour, &sch.Minute); err != nil {
		return sch, fmt.Errorf("SUMMARY_SCHEDULE %q: erwartet z.B. \"Sun 10:00\" oder \"off\"", s)
	}
	d, ok := weekdays[strings.ToLower(day)]
	if !ok || sch.Hour > 23 || sch.Minute > 59 || sch.Hour < 0 || sch.Minute < 0 {
		return sch, fmt.Errorf("SUMMARY_SCHEDULE %q: erwartet z.B. \"Sun 10:00\" oder \"off\"", s)
	}
	sch.Day = d
	return sch, nil
}

// Next is the first scheduled moment strictly after `after`, in after's
// location, so the wall-clock time holds across DST changes.
func (s Schedule) Next(after time.Time) time.Time {
	y, m, d := after.Date()
	for i := 0; i <= 7; i++ {
		c := time.Date(y, m, d+i, s.Hour, s.Minute, 0, 0, after.Location())
		if c.Weekday() == s.Day && c.After(after) {
			return c
		}
	}
	panic("unreachable")
}

// BuildSummary lists every watched item's price, lowest price and stock.
func BuildSummary(items []string, st *State) Notification {
	var lines []string
	if len(items) == 0 {
		lines = append(lines, "Keine Artikel in items.txt, es wird nichts beobachtet.")
	}
	for _, url := range items {
		it := st.Items[url]
		if it == nil || it.LastSuccess.IsZero() {
			line := url + ": noch keine Daten"
			if it != nil && it.LastError != "" {
				line += " (" + it.LastError + ")"
			}
			lines = append(lines, line)
			continue
		}
		line := fmt.Sprintf("%s: %s (Tiefstpreis %s), %s", it.Name, FormatEUR(it.Price), FormatEUR(it.Lowest), stockText(it.InStock))
		if it.Failures > 0 {
			line += fmt.Sprintf(", zuletzt %d× fehlgeschlagen", it.Failures)
		}
		lines = append(lines, line)
	}
	return Notification{
		Title:    "Westwing-Wochenübersicht",
		Message:  strings.Join(lines, "\n"),
		Priority: 2,
		Tags:     []string{"bar_chart"},
	}
}

// Jitter spreads d by ±15 %, so checks don't hit Westwing on a fixed beat.
func Jitter(d time.Duration, r *rand.Rand) time.Duration {
	return time.Duration(float64(d) * (0.85 + 0.3*r.Float64()))
}
