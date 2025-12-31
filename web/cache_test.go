package web

import (
	"testing"
	"time"
)

func TestCache_SetGet(t *testing.T) {
	c := NewCache(1 * time.Minute)

	c.Set("key1", "value1")
	c.Set("key2", 123)
	c.Set("key3", []string{"a", "b", "c"})

	val, ok := c.Get("key1")
	if !ok {
		t.Error("expected key1 to exist")
	}
	if val != "value1" {
		t.Errorf("expected 'value1', got %v", val)
	}

	val, ok = c.Get("key2")
	if !ok {
		t.Error("expected key2 to exist")
	}
	if val != 123 {
		t.Errorf("expected 123, got %v", val)
	}

	val, ok = c.Get("key3")
	if !ok {
		t.Error("expected key3 to exist")
	}
	arr, _ := val.([]string)
	if len(arr) != 3 {
		t.Errorf("expected 3 elements, got %d", len(arr))
	}
}

func TestCache_NotFound(t *testing.T) {
	c := NewCache(1 * time.Minute)

	_, ok := c.Get("nonexistent")
	if ok {
		t.Error("expected key to not exist")
	}
}

func TestCache_Expiry(t *testing.T) {
	c := NewCache(50 * time.Millisecond)

	c.Set("key", "value")

	val, ok := c.Get("key")
	if !ok {
		t.Error("expected key to exist immediately after setting")
	}
	if val != "value" {
		t.Errorf("expected 'value', got %v", val)
	}

	time.Sleep(100 * time.Millisecond)

	_, ok = c.Get("key")
	if ok {
		t.Error("expected key to be expired")
	}
}

func TestCache_Clear(t *testing.T) {
	c := NewCache(1 * time.Minute)

	c.Set("key1", "value1")
	c.Set("key2", "value2")

	if c.Size() != 2 {
		t.Errorf("expected size 2, got %d", c.Size())
	}

	c.Clear()

	if c.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", c.Size())
	}

	_, ok := c.Get("key1")
	if ok {
		t.Error("expected key1 to not exist after clear")
	}
}

func TestCache_Size(t *testing.T) {
	c := NewCache(1 * time.Minute)

	if c.Size() != 0 {
		t.Errorf("expected empty cache, got size %d", c.Size())
	}

	c.Set("key1", "value1")
	if c.Size() != 1 {
		t.Errorf("expected size 1, got %d", c.Size())
	}

	c.Set("key2", "value2")
	if c.Size() != 2 {
		t.Errorf("expected size 2, got %d", c.Size())
	}

	c.Set("key1", "updated")
	if c.Size() != 2 {
		t.Errorf("expected size 2 after update, got %d", c.Size())
	}
}

func TestCache_Overwrite(t *testing.T) {
	c := NewCache(1 * time.Minute)

	c.Set("key", "value1")
	val, _ := c.Get("key")
	if val != "value1" {
		t.Errorf("expected 'value1', got %v", val)
	}

	c.Set("key", "value2")
	val, _ = c.Get("key")
	if val != "value2" {
		t.Errorf("expected 'value2' after overwrite, got %v", val)
	}
}
