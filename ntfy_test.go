package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewNtfy(t *testing.T) {
	n, err := NewNtfy("https://ntfy.sh/westwing-abc")
	if err != nil {
		t.Fatal(err)
	}
	if n.Server != "https://ntfy.sh" || n.Topic != "westwing-abc" {
		t.Errorf("got server %q topic %q", n.Server, n.Topic)
	}
	if _, err := NewNtfy("https://ntfy.sh/"); err == nil {
		t.Error("want error for URL without topic")
	}
}

func TestNtfySendsJSON(t *testing.T) {
	var got map[string]any
	var method, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		json.NewDecoder(r.Body).Decode(&got)
	}))
	defer srv.Close()

	n, err := NewNtfy(srv.URL + "/abc")
	if err != nil {
		t.Fatal(err)
	}
	err = n.Send(context.Background(), Notification{Title: "Preis gesunken: Möbel 899,00 €", Message: "m", Priority: 4, Tags: []string{"x"}, Click: "https://c", Attach: "https://a"})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || path != "/" {
		t.Errorf("request %s %s", method, path)
	}
	if got["topic"] != "abc" || got["title"] != "Preis gesunken: Möbel 899,00 €" || got["priority"] != float64(4) || got["click"] != "https://c" || got["attach"] != "https://a" {
		t.Errorf("body %v", got)
	}
}

func TestNtfyErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusTooManyRequests)
	}))
	defer srv.Close()
	n, _ := NewNtfy(srv.URL + "/abc")
	if err := n.Send(context.Background(), Notification{Title: "t"}); err == nil {
		t.Error("want error on HTTP 429")
	}
}
