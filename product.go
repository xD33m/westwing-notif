package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Product is what one check reads from a Westwing product page.
type Product struct {
	Name     string
	Price    float64
	Currency string
	InStock  bool
	Image    string
}

var ldJSONRe = regexp.MustCompile(`(?is)<script[^>]*type=["']application/ld\+json["'][^>]*>(.*?)</script>`)

// ParseProduct reads the schema.org Product from the page's JSON-LD.
// Several offers (variants) resolve to the lowest price, so the same page
// always yields the same price.
func ParseProduct(html []byte) (Product, error) {
	for _, m := range ldJSONRe.FindAllSubmatch(html, -1) {
		var doc any
		if err := json.Unmarshal(m[1], &doc); err != nil {
			continue
		}
		if obj := findProduct(doc); obj != nil {
			return productFrom(obj)
		}
	}
	return Product{}, errors.New("keine Produktdaten auf der Seite gefunden")
}

func findProduct(v any) map[string]any {
	switch v := v.(type) {
	case map[string]any:
		if hasType(v, "Product") {
			return v
		}
		if g, ok := v["@graph"]; ok {
			return findProduct(g)
		}
	case []any:
		for _, e := range v {
			if p := findProduct(e); p != nil {
				return p
			}
		}
	}
	return nil
}

func hasType(obj map[string]any, want string) bool {
	switch t := obj["@type"].(type) {
	case string:
		return t == want
	case []any:
		for _, e := range t {
			if s, _ := e.(string); s == want {
				return true
			}
		}
	}
	return false
}

func productFrom(obj map[string]any) (Product, error) {
	p := Product{Price: -1}
	p.Name, _ = obj["name"].(string)
	p.Image = firstImage(obj["image"])

	var offers []any
	switch o := obj["offers"].(type) {
	case []any:
		offers = o
	case map[string]any:
		offers = []any{o}
	}
	for _, o := range offers {
		offer, ok := o.(map[string]any)
		if !ok {
			continue
		}
		price, ok := number(offer["price"])
		if !ok {
			price, ok = number(offer["lowPrice"])
		}
		if !ok || price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
			continue
		}
		if p.Price < 0 || price < p.Price {
			p.Price = price
			if c, _ := offer["priceCurrency"].(string); c != "" {
				p.Currency = c
			}
			// Stock belongs to the offer whose price is reported.
			a, _ := offer["availability"].(string)
			p.InStock = strings.HasSuffix(a, "InStock")
		}
	}
	if p.Price <= 0 {
		return Product{}, fmt.Errorf("kein gültiger Preis in den Produktdaten (%q)", p.Name)
	}
	if p.Currency == "" {
		p.Currency = "EUR"
	}
	return p, nil
}

func number(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f, err == nil
	}
	return 0, false
}

func firstImage(v any) string {
	switch i := v.(type) {
	case string:
		return i
	case []any:
		for _, e := range i {
			if s := firstImage(e); s != "" {
				return s
			}
		}
	case map[string]any:
		if s, _ := i["contentUrl"].(string); s != "" {
			return s
		}
		s, _ := i["url"].(string)
		return s
	}
	return ""
}
