package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strings"
	"time"
)

// failureThreshold is how many failed checks in a row trigger one alert.
const failureThreshold = 3

// OnSuccess folds a successful check into the item's state and returns the
// pushes it warrants. prev is nil (or never succeeded) on first sight.
func OnSuccess(prev *ItemState, url string, p Product, now time.Time) (ItemState, []Notification) {
	next := ItemState{
		Name:        p.Name,
		Price:       p.Price,
		Currency:    p.Currency,
		InStock:     p.InStock,
		Lowest:      p.Price,
		LowestAt:    now,
		LastSuccess: now,
	}
	name := displayName(p.Name, url)

	if prev == nil || prev.LastSuccess.IsZero() {
		return next, []Notification{{
			Title:    "Beobachte jetzt: " + name,
			Message:  fmt.Sprintf("Aktueller Preis: %s · %s", FormatEUR(p.Price), stockText(p.InStock)),
			Priority: 2,
			Tags:     []string{"eyes"},
			Click:    url,
		}}
	}

	if prev.Lowest > 0 && prev.Lowest <= p.Price {
		next.Lowest, next.LowestAt = prev.Lowest, prev.LowestAt
	}

	var ns []Notification
	if prev.FailureNotified {
		ns = append(ns, Notification{
			Title:    name + " wieder erreichbar",
			Message:  fmt.Sprintf("Prüfung läuft wieder. Preis: %s", FormatEUR(p.Price)),
			Priority: 3,
			Tags:     []string{"white_check_mark"},
			Click:    url,
		})
	}
	if p.Price < prev.Price {
		pct := (p.Price - prev.Price) / prev.Price * 100
		msg := fmt.Sprintf("%s → %s (%.0f %%)", FormatEUR(prev.Price), FormatEUR(p.Price), pct)
		if p.Price < prev.Lowest {
			msg += "\nNeuer Tiefstpreis!"
		} else {
			msg += fmt.Sprintf("\nTiefster bisher: %s", FormatEUR(prev.Lowest))
		}
		if !p.InStock {
			msg += "\nAktuell ausverkauft."
		}
		ns = append(ns, Notification{
			Title:    "Preis gesunken: " + name,
			Message:  msg,
			Priority: 4,
			Tags:     []string{"chart_with_downwards_trend"},
			Click:    url,
			Attach:   p.Image,
		})
	}
	if prev.InStock && !p.InStock {
		ns = append(ns, Notification{
			Title:    name + " ist ausverkauft",
			Message:  "Zuletzt " + FormatEUR(p.Price),
			Priority: 3,
			Tags:     []string{"x"},
			Click:    url,
		})
	}
	if !prev.InStock && p.InStock {
		ns = append(ns, Notification{
			Title:    name + " ist wieder verfügbar",
			Message:  "Preis: " + FormatEUR(p.Price),
			Priority: 3,
			Tags:     []string{"package"},
			Click:    url,
		})
	}
	return next, ns
}

// OnFailure counts a failed check; the third in a row sends one alert.
func OnFailure(prev *ItemState, url string, err error, now time.Time) (ItemState, []Notification) {
	var next ItemState
	if prev != nil {
		next = *prev
	}
	next.Failures++
	next.LastError = err.Error()
	if next.Failures < failureThreshold || next.FailureNotified {
		return next, nil
	}
	next.FailureNotified = true
	return next, []Notification{{
		Title:    "Prüfung schlägt fehl: " + displayName(next.Name, url),
		Message:  fmt.Sprintf("%d× hintereinander fehlgeschlagen: %s", next.Failures, err),
		Priority: 3,
		Tags:     []string{"warning"},
		Click:    url,
	}}
}

// FormatEUR renders 1049 as "1.049,00 €".
func FormatEUR(v float64) string {
	cents := int64(math.Round(v * 100))
	s := fmt.Sprintf("%d", cents/100)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return fmt.Sprintf("%s,%02d €", b.String(), cents%100)
}

func displayName(name, url string) string {
	if name != "" {
		return name
	}
	return url
}

func stockText(inStock bool) string {
	if inStock {
		return "auf Lager"
	}
	return "ausverkauft"
}

// Fetcher downloads one product page.
type Fetcher func(ctx context.Context, url string) ([]byte, error)

// HTTPFetcher fetches pages the way a normal browser would ask for them.
func HTTPFetcher(client *http.Client) Fetcher {
	return func(ctx context.Context, url string) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0 Safari/537.36")
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		req.Header.Set("Accept-Language", "de-DE,de;q=0.9")
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	}
}

// RunCycle checks every item once. An item's new state is kept only if all
// of its pushes went out, so a drop is re-detected after an ntfy outage.
func RunCycle(ctx context.Context, items []string, st *State, fetch Fetcher, n Notifier, historyPath string, pause time.Duration, now func() time.Time) {
	for i, url := range items {
		if i > 0 && pause > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(pause):
			}
		}
		prev := st.Items[url]
		var next ItemState
		var ns []Notification

		html, err := fetch(ctx, url)
		var p Product
		if err == nil {
			p, err = ParseProduct(html)
		}
		if err != nil {
			next, ns = OnFailure(prev, url, err, now())
			log.Printf("FEHLER %s: %v (%d× in Folge)", url, err, next.Failures)
		} else {
			next, ns = OnSuccess(prev, url, p, now())
			log.Printf("ok %s: %s %s", p.Name, FormatEUR(p.Price), stockText(p.InStock))
			if err := AppendHistory(historyPath, now(), url, p); err != nil {
				log.Printf("history: %v", err)
			}
		}

		delivered := true
		for _, msg := range ns {
			if err := n.Send(ctx, msg); err != nil {
				log.Printf("ntfy: %q nicht gesendet: %v", msg.Title, err)
				delivered = false
			}
		}
		if delivered {
			st.Items[url] = &next
		}
	}
}
