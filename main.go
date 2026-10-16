// westwing-notif watches Westwing product pages and pushes price drops,
// stock changes, failures and a weekly summary to ntfy.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	_ "time/tzdata" // scratch image has no zoneinfo

	_ "golang.org/x/crypto/x509roots/fallback" // scratch image has no CA certs
)

const itemPause = 5 * time.Second

type config struct {
	ntfyURL   string
	interval  time.Duration
	schedule  Schedule
	itemsFile string
	statePath string
	history   string
}

func loadConfig() (config, error) {
	c := config{
		ntfyURL:   os.Getenv("NTFY_URL"),
		itemsFile: env("ITEMS_FILE", "/config/items.txt"),
	}
	dataDir := env("DATA_DIR", "/data")
	c.statePath = filepath.Join(dataDir, "state.json")
	c.history = filepath.Join(dataDir, "history.csv")

	var err error
	if c.interval, err = time.ParseDuration(env("CHECK_INTERVAL", "2h")); err != nil || c.interval < time.Minute {
		return c, fmt.Errorf("CHECK_INTERVAL %q: erwartet Dauer ≥ 1m, z.B. 2h", os.Getenv("CHECK_INTERVAL"))
	}
	if c.schedule, err = ParseSchedule(env("SUMMARY_SCHEDULE", "Sun 10:00")); err != nil {
		return c, err
	}
	return c, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	log.SetFlags(0)
	cmd := "run"
	args := os.Args[1:]
	if len(args) > 0 && args[0][0] != '-' {
		cmd, args = args[0], args[1:]
	}
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}
	switch cmd {
	case "run":
		fs := flag.NewFlagSet("run", flag.ExitOnError)
		once := fs.Bool("once", false, "einen Durchlauf machen und beenden")
		fs.Parse(args)
		if err := run(cfg, *once); err != nil {
			log.Fatal(err)
		}
	case "healthcheck":
		if err := healthcheck(cfg, time.Now()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		log.Fatalf("unbekannter Befehl %q (run, healthcheck)", cmd)
	}
}

func run(cfg config, once bool) error {
	notifier, err := NewNtfy(cfg.ntfyURL)
	if err != nil {
		return err
	}
	st, err := LoadState(cfg.statePath)
	if err != nil {
		bad := cfg.statePath + ".bad"
		log.Printf("state unlesbar (%v), verschoben nach %s, starte neu", err, bad)
		os.Rename(cfg.statePath, bad)
		st, _ = LoadState(cfg.statePath)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	fetch := HTTPFetcher(&http.Client{Timeout: 60 * time.Second})
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	log.Printf("westwing-notif: Intervall %s, Übersicht %s, ntfy %s/%s", cfg.interval, scheduleText(cfg.schedule), notifier.Server, notifier.Topic)

	nextCheck := time.Now()
	var nextSummary time.Time
	if !cfg.schedule.Off {
		if st.LastSummary.IsZero() {
			nextSummary = cfg.schedule.Next(time.Now())
		} else {
			nextSummary = cfg.schedule.Next(st.LastSummary.In(time.Local))
		}
	}

	for {
		if !time.Now().Before(nextCheck) {
			items, err := ReadItems(cfg.itemsFile)
			if err != nil {
				log.Printf("items: %v", err)
			}
			RunCycle(ctx, items, st, fetch, notifier, cfg.history, itemPause, time.Now)
			if err := st.Save(cfg.statePath); err != nil {
				log.Printf("state: %v", err)
			}
			if once || ctx.Err() != nil {
				return nil
			}
			nextCheck = time.Now().Add(Jitter(cfg.interval, rng))
			log.Printf("nächste Prüfung %s", nextCheck.Format("Mon 02.01. 15:04"))
		}
		if !nextSummary.IsZero() && !time.Now().Before(nextSummary) {
			items, _ := ReadItems(cfg.itemsFile)
			if err := notifier.Send(ctx, BuildSummary(items, st)); err != nil {
				log.Printf("Wochenübersicht nicht gesendet: %v", err)
				nextSummary = time.Now().Add(10 * time.Minute)
			} else {
				st.LastSummary = time.Now()
				st.Save(cfg.statePath)
				nextSummary = cfg.schedule.Next(time.Now())
				log.Printf("Wochenübersicht gesendet")
			}
		}

		wake := nextCheck
		if !nextSummary.IsZero() && nextSummary.Before(wake) {
			wake = nextSummary
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Until(wake)):
		}
	}
}

func scheduleText(s Schedule) string {
	if s.Off {
		return "aus"
	}
	return fmt.Sprintf("%s %02d:%02d", s.Day, s.Hour, s.Minute)
}

// healthcheck fails when no item has been checked successfully within
// three intervals: the container is up but not doing its job.
func healthcheck(cfg config, now time.Time) error {
	st, err := LoadState(cfg.statePath)
	if err != nil {
		return err
	}
	var last time.Time
	for _, it := range st.Items {
		if it.LastSuccess.After(last) {
			last = it.LastSuccess
		}
	}
	if last.IsZero() {
		return fmt.Errorf("noch keine erfolgreiche Prüfung")
	}
	if age := now.Sub(last); age > 3*cfg.interval {
		return fmt.Errorf("letzte erfolgreiche Prüfung vor %s", age.Round(time.Minute))
	}
	return nil
}
