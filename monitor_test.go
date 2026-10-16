package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const libbyURL = "https://www.westwing.de/holz-highboard-libby-159713.html"

var t0 = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

func libby(price float64, inStock bool) Product {
	return Product{Name: "Holz-Highboard Libby", Price: price, Currency: "EUR", InStock: inStock, Image: "https://cdn.example/libby.jpg"}
}

func seen(price, lowest float64, inStock bool) *ItemState {
	return &ItemState{Name: "Holz-Highboard Libby", Price: price, Currency: "EUR", InStock: inStock, Lowest: lowest, LowestAt: t0.Add(-48 * time.Hour), LastSuccess: t0.Add(-2 * time.Hour)}
}

func TestFirstSightNotifiesWatching(t *testing.T) {
	st, ns := OnSuccess(nil, libbyURL, libby(1049, true), t0)
	if len(ns) != 1 {
		t.Fatalf("want 1 notification, got %d", len(ns))
	}
	n := ns[0]
	if n.Priority != 2 || !strings.Contains(n.Title, "Libby") || !strings.Contains(n.Message, "1.049,00 €") {
		t.Errorf("unexpected notification %+v", n)
	}
	if st.Lowest != 1049 || st.Price != 1049 || !st.LastSuccess.Equal(t0) {
		t.Errorf("unexpected state %+v", st)
	}
}

func TestDropNotifies(t *testing.T) {
	st, ns := OnSuccess(seen(1049, 1049, true), libbyURL, libby(899, true), t0)
	if len(ns) != 1 {
		t.Fatalf("want 1 notification, got %d", len(ns))
	}
	n := ns[0]
	if n.Priority != 4 || len(n.Tags) == 0 || n.Tags[0] != "chart_with_downwards_trend" {
		t.Errorf("priority/tags = %d %v", n.Priority, n.Tags)
	}
	for _, want := range []string{"1.049,00 €", "899,00 €", "-14 %", "Tiefstpreis"} {
		if !strings.Contains(n.Message, want) {
			t.Errorf("message %q lacks %q", n.Message, want)
		}
	}
	if n.Click != libbyURL || n.Attach != "https://cdn.example/libby.jpg" {
		t.Errorf("click/attach = %q %q", n.Click, n.Attach)
	}
	if st.Price != 899 || st.Lowest != 899 || !st.LowestAt.Equal(t0) {
		t.Errorf("unexpected state %+v", st)
	}
}

func TestDropAfterRiseNotifies(t *testing.T) {
	_, ns := OnSuccess(seen(1099, 1049, true), libbyURL, libby(1049, true), t0)
	if len(ns) != 1 {
		t.Fatalf("want 1 notification, got %d", len(ns))
	}
	if strings.Contains(ns[0].Message, "Tiefstpreis") {
		t.Errorf("message %q must not claim a new low", ns[0].Message)
	}
}

func TestRiseIsSilent(t *testing.T) {
	st, ns := OnSuccess(seen(1049, 1049, true), libbyURL, libby(1099, true), t0)
	if len(ns) != 0 {
		t.Fatalf("want no notification, got %+v", ns)
	}
	if st.Price != 1099 || st.Lowest != 1049 {
		t.Errorf("unexpected state %+v", st)
	}
}

func TestStockChanges(t *testing.T) {
	_, ns := OnSuccess(seen(1049, 1049, true), libbyURL, libby(1049, false), t0)
	if len(ns) != 1 || !strings.Contains(ns[0].Title, "ausverkauft") {
		t.Errorf("in → out: got %+v", ns)
	}
	_, ns = OnSuccess(seen(1049, 1049, false), libbyURL, libby(1049, true), t0)
	if len(ns) != 1 || !strings.Contains(ns[0].Title, "wieder verfügbar") {
		t.Errorf("out → in: got %+v", ns)
	}
}

func TestDropWhileOutOfStock(t *testing.T) {
	_, ns := OnSuccess(seen(1049, 1049, false), libbyURL, libby(899, false), t0)
	if len(ns) != 1 || !strings.Contains(ns[0].Message, "ausverkauft") {
		t.Errorf("got %+v", ns)
	}
}

func TestFailureThreshold(t *testing.T) {
	st := *seen(1049, 1049, true)
	boom := errors.New("HTTP 403")
	var ns []Notification
	for i := 1; i <= 2; i++ {
		st, ns = OnFailure(&st, libbyURL, boom, t0)
		if len(ns) != 0 {
			t.Fatalf("failure %d: want silence, got %+v", i, ns)
		}
	}
	st, ns = OnFailure(&st, libbyURL, boom, t0)
	if len(ns) != 1 || !strings.Contains(ns[0].Message, "HTTP 403") || !st.FailureNotified {
		t.Fatalf("failure 3: got %+v state %+v", ns, st)
	}
	st, ns = OnFailure(&st, libbyURL, boom, t0)
	if len(ns) != 0 {
		t.Fatalf("failure 4: want silence, got %+v", ns)
	}
	st, ns = OnSuccess(&st, libbyURL, libby(1049, true), t0)
	if len(ns) != 1 || !strings.Contains(ns[0].Title, "wieder erreichbar") {
		t.Fatalf("recovery: got %+v", ns)
	}
	if st.Failures != 0 || st.FailureNotified || st.LastError != "" {
		t.Errorf("recovery state %+v", st)
	}
}

func TestFormatEUR(t *testing.T) {
	for in, want := range map[float64]string{1049: "1.049,00 €", 899.5: "899,50 €", 12345.678: "12.345,68 €"} {
		if got := FormatEUR(in); got != want {
			t.Errorf("FormatEUR(%v) = %q, want %q", in, got, want)
		}
	}
}

type fakeNotifier struct {
	err  error
	sent []Notification
}

func (f *fakeNotifier) Send(_ context.Context, n Notification) error {
	f.sent = append(f.sent, n)
	return f.err
}

func TestCycleKeepsStateWhenPushFails(t *testing.T) {
	st := &State{Items: map[string]*ItemState{libbyURL: seen(1049, 1049, true)}}
	fetch := func(context.Context, string) ([]byte, error) {
		return ldPage(`{"@type":"Product","name":"Holz-Highboard Libby","offers":{"price":899,"priceCurrency":"EUR","availability":"https://schema.org/InStock"}}`), nil
	}
	n := &fakeNotifier{err: errors.New("ntfy down")}
	RunCycle(context.Background(), []string{libbyURL}, st, fetch, n, filepath.Join(t.TempDir(), "h.csv"), 0, func() time.Time { return t0 })
	if len(n.sent) != 1 {
		t.Fatalf("want 1 send attempt, got %d", len(n.sent))
	}
	if st.Items[libbyURL].Price != 1049 {
		t.Errorf("state advanced to %v despite failed push", st.Items[libbyURL].Price)
	}

	n.err = nil
	RunCycle(context.Background(), []string{libbyURL}, st, fetch, n, filepath.Join(t.TempDir(), "h.csv"), 0, func() time.Time { return t0 })
	if len(n.sent) != 2 || st.Items[libbyURL].Price != 899 {
		t.Errorf("retry: sent %d, price %v", len(n.sent), st.Items[libbyURL].Price)
	}
}
