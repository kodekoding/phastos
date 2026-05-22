package common

import (
	"bytes"
	"sync"
	"testing"
)

func TestGetBufferWithNilData(t *testing.T) {
	b := GetBuffer(nil)
	if b == nil {
		t.Fatal("expected non-nil buffer")
	}
	if b.Len() != 0 {
		t.Errorf("expected empty buffer, got len=%d", b.Len())
	}
	PutBuffer(b)
}

func TestGetBufferWithData(t *testing.T) {
	data := []byte("hello world")
	b := GetBuffer(data)
	if b == nil {
		t.Fatal("expected non-nil buffer")
	}
	if b.String() != "hello world" {
		t.Errorf("expected 'hello world', got %q", b.String())
	}
	PutBuffer(b)
}

func TestPutBufferResetsState(t *testing.T) {
	data := []byte("test data")
	b := GetBuffer(data)
	PutBuffer(b)

	// Get it again from the pool - should be reset
	b2 := GetBuffer(nil)
	if b2.Len() != 0 {
		t.Errorf("expected empty buffer after Put, got len=%d", b2.Len())
	}
	PutBuffer(b2)
}

func TestGetBufferConcurrentAccess(t *testing.T) {
	const workers = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				data := []byte{byte(id), byte(j)}
				b := GetBuffer(data)
				_ = b.String()
				PutBuffer(b)
			}
		}(i)
	}

	wg.Wait()
}

func TestGetBufferEmptySlice(t *testing.T) {
	b := GetBuffer([]byte{})
	if b == nil {
		t.Fatal("expected non-nil buffer")
	}
	if b.Len() != 0 {
		t.Errorf("expected empty buffer, got len=%d", b.Len())
	}
	PutBuffer(b)
}

func TestBufferPoolWriteAfterGet(t *testing.T) {
	b := GetBuffer(nil)
	b.WriteString("added")
	if b.String() != "added" {
		t.Errorf("expected 'added', got %q", b.String())
	}
	PutBuffer(b)

	// Verify reset after put+get
	b2 := GetBuffer(nil)
	if b2.Len() != 0 {
		t.Errorf("expected empty buffer after pool return, got len=%d", b2.Len())
	}
	PutBuffer(b2)
}

func TestGetBufferCompareBytes(t *testing.T) {
	data := []byte("compare me")
	b := GetBuffer(data)
	if !bytes.Equal(b.Bytes(), data) {
		t.Errorf("buffer bytes do not match input data")
	}
	PutBuffer(b)
}
