package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sensepost/gowitness/pkg/models"
)

func TestResultHasTechnologyCaseInsensitive(t *testing.T) {
	result := &models.Result{
		Technologies: []models.Technology{
			{Value: "nginx"},
			{Value: "WordPress"},
		},
	}

	if !resultHasTechnology(result, "NGINX") {
		t.Fatal("expected technology match to be case-insensitive")
	}

	if resultHasTechnology(result, "apache") {
		t.Fatal("did not expect apache to match")
	}
}

func TestDomainFromURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "http domain", input: "http://example.com:8080/a", expected: "example.com"},
		{name: "https uppercase", input: "https://EXAMPLE.COM/path", expected: "example.com"},
		{name: "ipv6", input: "https://[2001:db8::1]:443/", expected: "2001:db8::1"},
		{name: "invalid", input: "not-a-url", expected: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := domainFromURL(test.input); got != test.expected {
				t.Fatalf("expected %q, got %q", test.expected, got)
			}
		})
	}
}

func TestUniqueDomains(t *testing.T) {
	results := []*models.Result{
		{URL: "http://example.com", FinalURL: "https://www.example.com/login"},
		{URL: "https://www.example.com/other"},
		{URL: "https://blog.example.com"},
		{URL: "not-a-url"},
	}

	got := uniqueDomains(results)
	expected := []string{"blog.example.com", "www.example.com"}

	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

func TestWriteDomains(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "domains.txt")

	if err := writeDomains(path, []string{"a.example", "b.example"}); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}

	if string(bytes) != "a.example\nb.example\n" {
		t.Fatalf("unexpected file contents: %q", string(bytes))
	}
}
