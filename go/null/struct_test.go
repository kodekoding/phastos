package null

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- String ---

func TestStringFrom(t *testing.T) {
	s := StringFrom("hello")
	assert.Equal(t, "hello", s.String.String)
	assert.True(t, s.Valid)
	assert.False(t, s.NullContent)
	assert.Equal(t, "hello", s.Value)
}

func TestStringFromPtr(t *testing.T) {
	t.Run("non-nil pointer", func(t *testing.T) {
		val := "world"
		s := StringFromPtr(&val)
		assert.Equal(t, "world", s.String.String)
		assert.True(t, s.Valid)
	})

	t.Run("nil pointer", func(t *testing.T) {
		s := StringFromPtr(nil)
		assert.Equal(t, "", s.String.String)
		assert.False(t, s.Valid)
	})
}

func TestNullString(t *testing.T) {
	s := NullString(true)
	assert.True(t, s.Valid)
	assert.True(t, s.NullContent)
	assert.Equal(t, "", s.Value)
}

// --- Int ---

func TestIntFrom(t *testing.T) {
	i := IntFrom(42)
	assert.Equal(t, 42, i.Int.Int)
	assert.True(t, i.Valid)
	assert.False(t, i.NullContent)
	assert.Equal(t, 42, i.Value)
}

func TestIntFromPtr(t *testing.T) {
	t.Run("non-nil pointer", func(t *testing.T) {
		val := 99
		i := IntFromPtr(&val)
		assert.Equal(t, 99, i.Int.Int)
		assert.True(t, i.Valid)
	})

	t.Run("nil pointer", func(t *testing.T) {
		i := IntFromPtr(nil)
		assert.Equal(t, 0, i.Int.Int)
		assert.False(t, i.Valid)
	})
}

func TestNullInt(t *testing.T) {
	i := NullInt(true)
	assert.True(t, i.Valid)
	assert.True(t, i.NullContent)
	assert.Equal(t, 0, i.Value)
}

// --- Int64 ---

func TestInt64From(t *testing.T) {
	i := Int64From(123456789)
	assert.Equal(t, int64(123456789), i.Int64.Int64)
	assert.True(t, i.Valid)
	assert.False(t, i.NullContent)
}

func TestInt64FromPtr(t *testing.T) {
	t.Run("non-nil pointer", func(t *testing.T) {
		val := int64(100)
		i := Int64FromPtr(&val)
		assert.Equal(t, int64(100), i.Int64.Int64)
		assert.True(t, i.Valid)
	})

	t.Run("nil pointer", func(t *testing.T) {
		i := Int64FromPtr(nil)
		assert.Equal(t, int64(0), i.Int64.Int64)
		assert.False(t, i.Valid)
	})
}

// --- Float64 ---

func TestFloat64From(t *testing.T) {
	f := Float64From(3.14)
	assert.Equal(t, 3.14, f.Float64.Float64)
	assert.True(t, f.Valid)
	assert.False(t, f.NullContent)
}

func TestFloat64FromPtr(t *testing.T) {
	t.Run("non-nil pointer", func(t *testing.T) {
		val := 2.718
		f := Float64FromPtr(&val)
		assert.Equal(t, 2.718, f.Float64.Float64)
		assert.True(t, f.Valid)
	})

	t.Run("nil pointer", func(t *testing.T) {
		f := Float64FromPtr(nil)
		assert.Equal(t, float64(0), f.Float64.Float64)
		assert.False(t, f.Valid)
	})
}

// --- Bool ---

func TestBoolFrom(t *testing.T) {
	b := BoolFrom(true)
	assert.True(t, b.Bool.Bool)
	assert.True(t, b.Valid)
	assert.False(t, b.NullContent)

	b2 := BoolFrom(false)
	assert.False(t, b2.Bool.Bool)
	assert.True(t, b2.Valid)
	assert.False(t, b2.NullContent)
}

func TestBoolFromPtr(t *testing.T) {
	t.Run("non-nil pointer", func(t *testing.T) {
		val := true
		b := BoolFromPtr(&val)
		assert.True(t, b.Bool.Bool)
		assert.True(t, b.Valid)
	})

	t.Run("nil pointer", func(t *testing.T) {
		b := BoolFromPtr(nil)
		assert.False(t, b.Bool.Bool)
		assert.False(t, b.Valid)
	})
}

// --- Time ---

func TestTimeFrom(t *testing.T) {
	now := time.Now()
	tm := TimeFrom(now)
	assert.Equal(t, now, tm.Time.Time)
	assert.True(t, tm.Valid)
	assert.False(t, tm.NullContent)
}

func TestTimeFromPtr(t *testing.T) {
	t.Run("non-nil pointer", func(t *testing.T) {
		now := time.Now()
		tm := TimeFromPtr(&now)
		assert.Equal(t, now, tm.Time.Time)
		assert.True(t, tm.Valid)
	})

	t.Run("nil pointer", func(t *testing.T) {
		tm := TimeFromPtr(nil)
		assert.True(t, tm.Time.Time.IsZero())
		assert.False(t, tm.Valid)
	})
}

// --- Int8 ---

func TestInt8From(t *testing.T) {
	i := Int8From(8)
	assert.Equal(t, int8(8), i.Int8.Int8)
	assert.True(t, i.Valid)
	assert.False(t, i.NullContent)
}

func TestInt8FromPtr(t *testing.T) {
	val := int8(5)
	i := Int8FromPtr(&val)
	assert.Equal(t, int8(5), i.Int8.Int8)
	assert.True(t, i.Valid)

	i2 := Int8FromPtr(nil)
	assert.False(t, i2.Valid)
}

// --- Float32 ---

func TestFloat32From(t *testing.T) {
	f := Float32From(1.5)
	assert.Equal(t, float32(1.5), f.Float32.Float32)
	assert.True(t, f.Valid)
	assert.False(t, f.NullContent)
}

func TestFloat32FromPtr(t *testing.T) {
	val := float32(2.5)
	f := Float32FromPtr(&val)
	assert.Equal(t, float32(2.5), f.Float32.Float32)
	assert.True(t, f.Valid)

	f2 := Float32FromPtr(nil)
	assert.False(t, f2.Valid)
}

// --- Uint64 ---

func TestUint64From(t *testing.T) {
	u := Uint64From(999)
	assert.Equal(t, uint64(999), u.Uint64.Uint64)
	assert.True(t, u.Valid)
	assert.False(t, u.NullContent)
}

func TestUint64FromPtr(t *testing.T) {
	val := uint64(100)
	u := Uint64FromPtr(&val)
	assert.Equal(t, uint64(100), u.Uint64.Uint64)
	assert.True(t, u.Valid)

	u2 := Uint64FromPtr(nil)
	assert.False(t, u2.Valid)
}
