package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSummaryWithoutItemsSaysSo(t *testing.T) {
	n := BuildSummary(nil, &State{Items: map[string]*ItemState{}})
	if !strings.Contains(n.Message, "Keine Artikel") {
		t.Errorf("message = %q", n.Message)
	}
}

func TestParseSkipsInvalidOfferPrices(t *testing.T) {
	for _, jsonld := range []string{
		`{"@type":"Product","name":"A","offers":[{"price":0,"availability":"https://schema.org/InStock"},{"price":1049,"availability":"https://schema.org/InStock"}]}`,
		`{"@type":"Product","name":"A","offers":[{"price":"NaN"},{"price":1049,"availability":"https://schema.org/InStock"}]}`,
		`{"@type":"Product","name":"A","offers":[{"price":"Inf"},{"price":1049,"availability":"https://schema.org/InStock"}]}`,
	} {
		p, err := ParseProduct(ldPage(jsonld))
		if err != nil || p.Price != 1049 {
			t.Errorf("%s → %v, %v; want 1049", jsonld, p.Price, err)
		}
	}
}

func TestStockComesFromCheapestOffer(t *testing.T) {
	p, err := ParseProduct(ldPage(`{"@type":"Product","name":"A","offers":[{"price":899,"availability":"https://schema.org/OutOfStock"},{"price":999,"availability":"https://schema.org/InStock"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if p.Price != 899 || p.InStock {
		t.Errorf("got price %v inStock %v, want 899 out of stock", p.Price, p.InStock)
	}
}

func TestDropShowsAbsoluteChangeAndSmallPercent(t *testing.T) {
	_, ns := OnSuccess(seen(1049, 1049, true), libbyURL, libby(1045, true), t0)
	if len(ns) != 1 {
		t.Fatalf("got %d notifications", len(ns))
	}
	if !strings.Contains(ns[0].Message, "-4,00 €") || !strings.Contains(ns[0].Message, "-0,4 %") {
		t.Errorf("message = %q, want -4,00 € and -0,4 %%", ns[0].Message)
	}
	_, ns = OnSuccess(seen(1049, 1049, true), libbyURL, libby(899, true), t0)
	if !strings.Contains(ns[0].Message, "-150,00 €") || !strings.Contains(ns[0].Message, "-14 %") {
		t.Errorf("message = %q, want -150,00 € and -14 %%", ns[0].Message)
	}
}

func TestFetcherRejectsRedirectToOtherPage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/old.html", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/other.html", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/other.html", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html></html>")) })
	mux.HandleFunc("/ok.html", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>ok</html>")) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	fetch := HTTPFetcher(srv.Client())
	if _, err := fetch(context.Background(), srv.URL+"/old.html"); err == nil || !strings.Contains(err.Error(), "weitergeleitet") {
		t.Errorf("redirect: err = %v, want weitergeleitet", err)
	}
	if b, err := fetch(context.Background(), srv.URL+"/ok.html?utm=x"); err != nil || !strings.Contains(string(b), "ok") {
		t.Errorf("same page: %q, %v", b, err)
	}
}

func TestFetcherRejectsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "blocked", http.StatusForbidden) }))
	defer srv.Close()
	if _, err := HTTPFetcher(srv.Client())(context.Background(), srv.URL+"/x.html"); err == nil || !strings.Contains(err.Error(), "403") {
		t.Errorf("err = %v, want HTTP 403", err)
	}
}
