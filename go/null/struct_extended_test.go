package null

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- NewString ---

func TestNewString(t *testing.T) {
	s := NewString("hello", true, false)
	assert.Equal(t, "hello", s.String.String)
	assert.True(t, s.Valid)
	assert.False(t, s.NullContent)
	assert.Equal(t, "hello", s.Value)
}

func TestNewStringInvalid(t *testing.T) {
	s := NewString("", false, false)
	assert.False(t, s.Valid)
	assert.False(t, s.NullContent)
}

// --- Int16 ---

func TestInt16From(t *testing.T) {
	i := Int16From(16)
	assert.Equal(t, int16(16), i.Int16.Int16)
	assert.True(t, i.Valid)
	assert.False(t, i.NullContent)
	assert.Equal(t, int16(16), i.Value)
}

func TestInt16FromPtr(t *testing.T) {
	val := int16(10)
	i := Int16FromPtr(&val)
	assert.Equal(t, int16(10), i.Int16.Int16)
	assert.True(t, i.Valid)

	i2 := Int16FromPtr(nil)
	assert.False(t, i2.Valid)
}

func TestNullInt16(t *testing.T) {
	i := NullInt16(true)
	assert.True(t, i.Valid)
	assert.True(t, i.NullContent)
}

func TestNewInt16(t *testing.T) {
	i := NewInt16(32, true, false)
	assert.Equal(t, int16(32), i.Int16.Int16)
	assert.True(t, i.Valid)
	assert.False(t, i.NullContent)
}

// --- Int32 ---

func TestInt32From(t *testing.T) {
	i := Int32From(32000)
	assert.Equal(t, int32(32000), i.Int32.Int32)
	assert.True(t, i.Valid)
	assert.False(t, i.NullContent)
}

func TestInt32FromPtr(t *testing.T) {
	val := int32(100)
	i := Int32FromPtr(&val)
	assert.Equal(t, int32(100), i.Int32.Int32)
	assert.True(t, i.Valid)

	i2 := Int32FromPtr(nil)
	assert.False(t, i2.Valid)
}

func TestNullInt32(t *testing.T) {
	i := NullInt32(true)
	assert.True(t, i.Valid)
	assert.True(t, i.NullContent)
}

func TestNewInt32(t *testing.T) {
	i := NewInt32(42, true, true)
	assert.Equal(t, int32(42), i.Int32.Int32)
	assert.True(t, i.Valid)
	assert.True(t, i.NullContent)
}

// --- Uint ---

func TestUintFrom(t *testing.T) {
	u := UintFrom(42)
	assert.Equal(t, uint(42), u.Uint.Uint)
	assert.True(t, u.Valid)
	assert.False(t, u.NullContent)
}

func TestUintFromPtr(t *testing.T) {
	val := uint(99)
	u := UintFromPtr(&val)
	assert.Equal(t, uint(99), u.Uint.Uint)
	assert.True(t, u.Valid)

	u2 := UintFromPtr(nil)
	assert.False(t, u2.Valid)
}

func TestNullUint(t *testing.T) {
	u := NullUint(true)
	assert.True(t, u.Valid)
	assert.True(t, u.NullContent)
}

func TestNewUint(t *testing.T) {
	u := NewUint(100, true, false)
	assert.Equal(t, uint(100), u.Uint.Uint)
	assert.Equal(t, uint(100), u.Value)
}

// --- Uint8 ---

func TestUint8From(t *testing.T) {
	u := Uint8From(255)
	assert.Equal(t, uint8(255), u.Uint8.Uint8)
	assert.True(t, u.Valid)
}

func TestUint8FromPtr(t *testing.T) {
	val := uint8(128)
	u := Uint8FromPtr(&val)
	assert.Equal(t, uint8(128), u.Uint8.Uint8)
	assert.True(t, u.Valid)

	u2 := Uint8FromPtr(nil)
	assert.False(t, u2.Valid)
}

func TestNullUint8(t *testing.T) {
	u := NullUint8(true)
	assert.True(t, u.Valid)
	assert.True(t, u.NullContent)
}

func TestNewUint8(t *testing.T) {
	u := NewUint8(10, true, false)
	assert.Equal(t, uint8(10), u.Value)
}

// --- Uint16 ---

func TestUint16From(t *testing.T) {
	u := Uint16From(65535)
	assert.Equal(t, uint16(65535), u.Uint16.Uint16)
	assert.True(t, u.Valid)
}

func TestUint16FromPtr(t *testing.T) {
	val := uint16(1000)
	u := Uint16FromPtr(&val)
	assert.Equal(t, uint16(1000), u.Uint16.Uint16)
	assert.True(t, u.Valid)

	u2 := Uint16FromPtr(nil)
	assert.False(t, u2.Valid)
}

func TestNullUint16(t *testing.T) {
	u := NullUint16(true)
	assert.True(t, u.Valid)
	assert.True(t, u.NullContent)
}

func TestNewUint16(t *testing.T) {
	u := NewUint16(500, true, false)
	assert.Equal(t, uint16(500), u.Value)
}

// --- Uint32 ---

func TestUint32From(t *testing.T) {
	u := Uint32From(100000)
	assert.Equal(t, uint32(100000), u.Uint32.Uint32)
	assert.True(t, u.Valid)
}

func TestUint32FromPtr(t *testing.T) {
	val := uint32(999)
	u := Uint32FromPtr(&val)
	assert.Equal(t, uint32(999), u.Uint32.Uint32)
	assert.True(t, u.Valid)

	u2 := Uint32FromPtr(nil)
	assert.False(t, u2.Valid)
}

func TestNullUint32(t *testing.T) {
	u := NullUint32(true)
	assert.True(t, u.Valid)
	assert.True(t, u.NullContent)
}

func TestNewUint32(t *testing.T) {
	u := NewUint32(42, true, false)
	assert.Equal(t, uint32(42), u.Value)
}

// --- NewInt ---

func TestNewInt(t *testing.T) {
	i := NewInt(42, true, false)
	assert.Equal(t, 42, i.Int.Int)
	assert.True(t, i.Valid)
	assert.Equal(t, 42, i.Value)
}

func TestNewIntInvalid(t *testing.T) {
	i := NewInt(0, false, false)
	assert.False(t, i.Valid)
}

// --- NewInt8 ---

func TestNewInt8(t *testing.T) {
	i := NewInt8(8, true, true)
	assert.Equal(t, int8(8), i.Int8.Int8)
	assert.True(t, i.Valid)
	assert.True(t, i.NullContent)
}

// --- NewInt64 ---

func TestNewInt64(t *testing.T) {
	i := NewInt64(999, true, false)
	assert.Equal(t, int64(999), i.Int64.Int64)
	assert.True(t, i.Valid)
	assert.Equal(t, int64(999), i.Value)
}

// --- NewFloat32 ---

func TestNewFloat32(t *testing.T) {
	f := NewFloat32(1.5, true, false)
	assert.Equal(t, float32(1.5), f.Float32.Float32)
	assert.True(t, f.Valid)
	assert.Equal(t, float32(1.5), f.Value)
}

// --- NewFloat64 ---

func TestNewFloat64(t *testing.T) {
	f := NewFloat64(3.14, true, false)
	assert.Equal(t, 3.14, f.Float64.Float64)
	assert.True(t, f.Valid)
	assert.Equal(t, 3.14, f.Value)
}

// --- NewBool ---

func TestNewBool(t *testing.T) {
	b := NewBool(true, true, false)
	assert.True(t, b.Bool.Bool)
	assert.True(t, b.Valid)
	assert.False(t, b.NullContent)
}

func TestNullBool(t *testing.T) {
	b := NullBool(true)
	assert.True(t, b.Valid)
	assert.True(t, b.NullContent)
	assert.True(t, b.Bool.Bool) // when isNull=true, Bool=i (true)
}

// --- NewTime ---

func TestNewTime(t *testing.T) {
	now := time.Now()
	tm := NewTime(now, true, false)
	assert.Equal(t, now, tm.Time.Time)
	assert.True(t, tm.Valid)
	assert.Equal(t, now, tm.Value)
}

// --- NullTime ---

func TestNullTime(t *testing.T) {
	tm := NullTime(true)
	assert.True(t, tm.Valid)
	assert.True(t, tm.NullContent)
}

// --- NullInt8 ---

func TestNullInt8(t *testing.T) {
	i := NullInt8(true)
	assert.True(t, i.Valid)
	assert.True(t, i.NullContent)
}

// --- NullInt64 ---

func TestNullInt64(t *testing.T) {
	i := NullInt64(true)
	assert.True(t, i.Valid)
	assert.True(t, i.NullContent)
}

// --- NullUint64 ---

func TestNullUint64(t *testing.T) {
	u := NullUint64(true)
	assert.True(t, u.Valid)
	assert.True(t, u.NullContent)
}

// --- NullFloat32 ---

func TestNullFloat32(t *testing.T) {
	f := NullFloat32(true)
	assert.True(t, f.Valid)
	assert.True(t, f.NullContent)
}

// --- NullFloat64 ---

func TestNullFloat64(t *testing.T) {
	f := NullFloat64(true)
	assert.True(t, f.Valid)
	assert.True(t, f.NullContent)
}
