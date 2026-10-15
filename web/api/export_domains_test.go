package api

import (
	"reflect"
	"testing"

	"github.com/sensepost/gowitness/pkg/models"
)

func TestUniqueDomainsFromResults(t *testing.T) {
	results := []models.Result{
		{URL: "http://example.com", FinalURL: "https://www.example.com/login"},
		{URL: "https://www.example.com/portal"},
		{URL: "https://blog.example.com"},
		{URL: "not-a-url"},
	}

	got := uniqueDomainsFromResults(results)
	expected := []string{"blog.example.com", "www.example.com"}

	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

func TestDomainFromTargetURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "http with port", input: "http://Example.COM:8080/path", expected: "example.com"},
		{name: "ipv6", input: "https://[2001:db8::1]:8443", expected: "2001:db8::1"},
		{name: "invalid", input: "not-a-url", expected: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := domainFromTargetURL(test.input); got != test.expected {
				t.Fatalf("expected %q, got %q", test.expected, got)
			}
		})
	}
}

func TestSanitizeExportFileName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "simple", input: "Nginx", expected: "nginx"},
		{name: "spaces and punctuation", input: "Microsoft IIS/10.0", expected: "microsoft-iis-10.0"},
		{name: "empty", input: "   ", expected: "technology"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sanitizeExportFileName(test.input); got != test.expected {
				t.Fatalf("expected %q, got %q", test.expected, got)
			}
		})
	}
}
