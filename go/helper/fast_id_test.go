package helper

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateFastID(t *testing.T) {
	t.Run("should generate 15 character string", func(t *testing.T) {
		id := GenerateFastID()
		assert.Len(t, id, 15)
	})

	t.Run("should only contain valid characters from alphabet", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			id := GenerateFastID()
			for _, c := range id {
				assert.Contains(t, fastIDAlphabet, string(c), "character %c not in alphabet", c)
			}
		}
	})

	t.Run("should generate unique IDs on multiple calls", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 1000; i++ {
			id := GenerateFastID()
			assert.False(t, ids[id], "duplicate ID generated: %s", id)
			ids[id] = true
		}
	})

	t.Run("should not return empty string", func(t *testing.T) {
		id := GenerateFastID()
		assert.NotEmpty(t, id)
	})
}

func TestGenerateFastIDCounter(t *testing.T) {
	t.Run("should generate 15 character string", func(t *testing.T) {
		id := GenerateFastIDCounter()
		assert.Len(t, id, 15)
	})

	t.Run("should only contain hex characters", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			id := GenerateFastIDCounter()
			for _, c := range id {
				assert.Contains(t, hexDigits, string(c), "character %c not in hex digits", c)
			}
		}
	})

	t.Run("should generate sequential IDs", func(t *testing.T) {
		id1 := GenerateFastIDCounter()
		id2 := GenerateFastIDCounter()
		assert.NotEqual(t, id1, id2, "consecutive counter IDs should differ")
	})

	t.Run("should generate unique IDs across many calls", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 1000; i++ {
			id := GenerateFastIDCounter()
			assert.False(t, ids[id], "duplicate counter ID generated: %s", id)
			ids[id] = true
		}
	})
}

func TestGenerateFastIDCounterBytes(t *testing.T) {
	t.Run("should return 16-byte slice", func(t *testing.T) {
		bp := GenerateFastIDCounterBytes()
		assert.NotNil(t, bp)
		assert.Len(t, *bp, 16)
	})

	t.Run("should contain only hex characters", func(t *testing.T) {
		bp := GenerateFastIDCounterBytes()
		buf := *bp
		str := string(buf)
		for _, c := range str {
			assert.Contains(t, hexDigits, string(c))
		}
	})

	t.Run("should generate unique values across calls", func(t *testing.T) {
		bp1 := GenerateFastIDCounterBytes()
		bp2 := GenerateFastIDCounterBytes()
		assert.NotEqual(t, string(*bp1), string(*bp2))
		// Do not PutFastIDCounterBytes here — the pool buffers are still referenced
		// in this test, and returning them can cause race conditions with
		// concurrent tests that also use the pool.
	})
}

func TestPutFastIDCounterBytes(t *testing.T) {
	t.Run("should return byte slice to pool without panic", func(t *testing.T) {
		bp := GenerateFastIDCounterBytes()
		assert.NotPanics(t, func() {
			PutFastIDCounterBytes(bp)
		})
	})

	t.Run("should handle nil without panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			PutFastIDCounterBytes(nil)
		})
	})

	t.Run("pool round-trip: get and put work correctly", func(t *testing.T) {
		bp := GenerateFastIDCounterBytes()
		val := string(*bp)
		assert.Len(t, *bp, 16)
		assert.NotEmpty(t, val)

		// Put back — this should not panic
		PutFastIDCounterBytes(bp)

		// After putting back, get again still works
		bp2 := GenerateFastIDCounterBytes()
		assert.NotNil(t, bp2)
		assert.Len(t, *bp2, 16)
		PutFastIDCounterBytes(bp2)
	})
}

func TestGenerateFastID_Concurrent(t *testing.T) {
	t.Run("should be safe for concurrent use", func(t *testing.T) {
		results := make(chan string, 100)
		for i := 0; i < 100; i++ {
			go func() {
				results <- GenerateFastID()
			}()
		}
		ids := make(map[string]bool)
		for i := 0; i < 100; i++ {
			id := <-results
			assert.False(t, ids[id], "duplicate concurrent ID: %s", id)
			ids[id] = true
		}
	})
}

func TestGenerateFastIDCounter_Concurrent(t *testing.T) {
	t.Run("should be safe for concurrent use", func(t *testing.T) {
		results := make(chan string, 100)
		for i := 0; i < 100; i++ {
			go func() {
				results <- GenerateFastIDCounter()
			}()
		}
		ids := make(map[string]bool)
		for i := 0; i < 100; i++ {
			id := <-results
			assert.False(t, ids[id], "duplicate concurrent counter ID: %s", id)
			ids[id] = true
		}
	})
}

func TestGenerateFastID_AllCharsValid(t *testing.T) {
	t.Run("should use full alphabet including underscore and dash", func(t *testing.T) {
		// Generate many IDs and check that _ and - appear at least sometimes
		allChars := strings.Builder{}
		for i := 0; i < 500; i++ {
			allChars.WriteString(GenerateFastID())
		}
		combined := allChars.String()
		assert.Contains(t, combined, "_", "underscore should appear in generated IDs")
		assert.Contains(t, combined, "-", "dash should appear in generated IDs")
	})
}

func TestGenerateFastIDCounterBytes_NilPoolItem(t *testing.T) {
	t.Run("should handle nil item from pool", func(t *testing.T) {
		fastIDBufPool.Put(nil)
		bp := GenerateFastIDCounterBytes()
		assert.NotNil(t, bp)
		assert.Len(t, *bp, 16)
	})
}
