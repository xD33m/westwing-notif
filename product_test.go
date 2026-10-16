package main

import (
	"os"
	"strings"
	"testing"
)

func TestParseLibby(t *testing.T) {
	html, err := os.ReadFile("testdata/libby.html")
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseProduct(html)
	if err != nil {
		t.Fatalf("ParseProduct: %v", err)
	}
	if !strings.Contains(p.Name, "Libby") {
		t.Errorf("Name = %q, want it to contain Libby", p.Name)
	}
	if p.Price != 1049 {
		t.Errorf("Price = %v, want 1049", p.Price)
	}
	if p.Currency != "EUR" {
		t.Errorf("Currency = %q, want EUR", p.Currency)
	}
	if !p.InStock {
		t.Error("InStock = false, want true")
	}
	if !strings.HasPrefix(p.Image, "https://") {
		t.Errorf("Image = %q, want https URL", p.Image)
	}
}

func ldPage(jsonld string) []byte {
	return []byte(`<html><head><script type="application/ld+json">` + jsonld + `</script></head><body></body></html>`)
}

func TestParsePriceVariants(t *testing.T) {
	tests := []struct {
		name    string
		jsonld  string
		price   float64
		inStock bool
	}{
		{"number", `{"@type":"Product","name":"A","offers":{"@type":"Offer","price":899,"priceCurrency":"EUR","availability":"https://schema.org/InStock"}}`, 899, true},
		{"string", `{"@type":"Product","name":"A","offers":{"@type":"Offer","price":"899.00","priceCurrency":"EUR","availability":"https://schema.org/InStock"}}`, 899, true},
		{"array lowest", `{"@type":"Product","name":"A","offers":[{"@type":"Offer","price":999,"availability":"https://schema.org/InStock"},{"@type":"Offer","price":899,"availability":"https://schema.org/InStock"}]}`, 899, true},
		{"aggregate", `{"@type":"Product","name":"A","offers":{"@type":"AggregateOffer","lowPrice":899,"highPrice":999,"availability":"https://schema.org/InStock"}}`, 899, true},
		{"graph", `{"@context":"https://schema.org","@graph":[{"@type":"WebPage"},{"@type":"Product","name":"A","offers":[{"@type":"Offer","price":899,"availability":"https://schema.org/InStock"}]}]}`, 899, true},
		{"out of stock", `{"@type":"Product","name":"A","offers":{"@type":"Offer","price":899,"availability":"https://schema.org/OutOfStock"}}`, 899, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := ParseProduct(ldPage(tt.jsonld))
			if err != nil {
				t.Fatalf("ParseProduct: %v", err)
			}
			if p.Price != tt.price {
				t.Errorf("Price = %v, want %v", p.Price, tt.price)
			}
			if p.InStock != tt.inStock {
				t.Errorf("InStock = %v, want %v", p.InStock, tt.inStock)
			}
		})
	}
}

func TestParseRejectsPageWithoutProduct(t *testing.T) {
	if _, err := ParseProduct([]byte(`<html>Access denied</html>`)); err == nil {
		t.Fatal("want error for page without product data")
	}
}

func TestParseRejectsZeroPrice(t *testing.T) {
	_, err := ParseProduct(ldPage(`{"@type":"Product","name":"A","offers":{"@type":"Offer","price":0}}`))
	if err == nil {
		t.Fatal("want error for zero price")
	}
}
