package client

import (
	"errors"
	"testing"
	"time"
)

func TestResponseCache_Hit(t *testing.T) {
	cache := NewResponseCache(5 * time.Minute)
	calls := 0
	fetch := func() ([]byte, error) {
		calls++
		return []byte(`[{"id":1}]`), nil
	}

	data1, err := cache.GetOrFetch("key1", fetch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data2, err := cache.GetOrFetch("key1", fetch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls != 1 {
		t.Errorf("expected 1 fetch call (cache hit), got %d", calls)
	}
	if string(data1) != string(data2) {
		t.Error("expected same data from cache hit")
	}
}

func TestResponseCache_Miss(t *testing.T) {
	cache := NewResponseCache(5 * time.Minute)
	calls := 0
	fetch := func() ([]byte, error) {
		calls++
		return []byte(`[]`), nil
	}

	cache.GetOrFetch("key1", fetch)
	cache.GetOrFetch("key2", fetch)

	if calls != 2 {
		t.Errorf("expected 2 fetch calls (different keys), got %d", calls)
	}
}

func TestResponseCache_Expiry(t *testing.T) {
	cache := NewResponseCache(1 * time.Millisecond)
	calls := 0
	fetch := func() ([]byte, error) {
		calls++
		return []byte(`[]`), nil
	}

	cache.GetOrFetch("key1", fetch)
	time.Sleep(5 * time.Millisecond)
	cache.GetOrFetch("key1", fetch)

	if calls != 2 {
		t.Errorf("expected 2 fetch calls (expired entry), got %d", calls)
	}
}

func TestResponseCache_FetchError(t *testing.T) {
	cache := NewResponseCache(5 * time.Minute)
	fetch := func() ([]byte, error) {
		return nil, errors.New("upstream error")
	}

	_, err := cache.GetOrFetch("key1", fetch)
	if err == nil {
		t.Error("expected error from failed fetch")
	}
}
