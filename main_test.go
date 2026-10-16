package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestHealthcheck(t *testing.T) {
	cfg := config{interval: 2 * time.Hour, statePath: filepath.Join(t.TempDir(), "state.json")}
	if err := healthcheck(cfg, t0); err == nil {
		t.Error("no state yet: want unhealthy")
	}
	st := &State{Items: map[string]*ItemState{
		"a": {LastSuccess: t0.Add(-7 * time.Hour)},
		"b": {LastSuccess: t0.Add(-5 * time.Hour)},
	}}
	st.Save(cfg.statePath)
	if err := healthcheck(cfg, t0); err != nil {
		t.Errorf("success 5h ago with 2h interval: want healthy, got %v", err)
	}
	if err := healthcheck(cfg, t0.Add(2*time.Hour)); err == nil {
		t.Error("success 7h ago with 2h interval: want unhealthy")
	}
}
