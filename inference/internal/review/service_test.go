package review

import (
	"context"
	"errors"
	"testing"
)

type errorProvider struct{}

func (errorProvider) Review(context.Context, string) ([]Comment, error) {
	return nil, errors.New("provider unavailable")
}

func TestServiceCachesValidatedResults(t *testing.T) {
	expected := []Comment{{File: "main.go", Line: 4, Severity: SeverityHigh, Message: "Handle error", Category: "bug"}}
	provider := &MockProvider{Comments: expected}
	service, err := NewService(provider, NewLRUCache(2), 100)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		got, err := service.Review(context.Background(), "diff")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0] != expected[0] {
			t.Fatalf("Review() = %#v, want %#v", got, expected)
		}
	}
	if provider.Calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.Calls)
	}
}

func TestServiceReturnsProviderAndValidationErrors(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
	}{
		{name: "provider error", provider: errorProvider{}},
		{name: "invalid model comment", provider: &MockProvider{Comments: []Comment{{File: "main.go", Line: 0}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, err := NewService(test.provider, NewLRUCache(1), 10)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.Review(context.Background(), "diff"); err == nil {
				t.Fatal("Review() expected an error")
			}
		})
	}
}

func TestLRUCacheEvictsLeastRecentlyUsed(t *testing.T) {
	cache := NewLRUCache(2)
	cache.Add("first", []Comment{{File: "a", Line: 1}})
	cache.Add("second", []Comment{{File: "b", Line: 1}})
	if _, ok := cache.Get("first"); !ok {
		t.Fatal("expected first entry")
	}
	cache.Add("third", nil)
	if _, ok := cache.Get("second"); ok {
		t.Fatal("least-recently-used entry was not evicted")
	}
}

func TestDiffHash(t *testing.T) {
	if DiffHash("a") == DiffHash("b") {
		t.Fatal("different diffs must have different hashes")
	}
}
