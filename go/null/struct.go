package null

// extended version from "github.com/volatiletech/null" to set null content
import (
	"github.com/volatiletech/null"
	"time"
)

type String struct {
	null.String
	NullContent bool
}

// NullString creates a new String with set null content
func NullString(s bool) String {
	return NewString("", true, s)
}

// StringFrom creates a new String that wil, falsel never be blank.
func StringFrom(s string) String {
	return NewString(s, true, false)
}

// StringFromPtr creates a new String that be null if s is nil.
func StringFromPtr(s *string) String {
	if s == nil {
		return NewString("", false, false)
	}
	return NewString(*s, true, false)
}

// NewString creates a new String
func NewString(s string, valid bool, isNull bool) String {
	return String{
		String: null.String{
			String: s,
			Valid:  valid,
		},
		NullContent: isNull,
	}
}

type Int struct {
	null.Int
	NullContent bool
}

// NullInt creates a new Int with set null content
func NullInt(s bool) Int {
	return NewInt(0, true, s)
}

// IntFrom creates a new Int that will never be blank.
func IntFrom(s int) Int {
	return NewInt(s, true, false)
}

// IntFromPtr creates a new Int that be null if s is nil.
func IntFromPtr(s *int) Int {
	if s == nil {
		return NewInt(0, false, false)
	}
	return NewInt(*s, true, false)
}

// NewInt creates a new Int
func NewInt(i int, valid bool, isNull bool) Int {
	return Int{
		Int: null.Int{
			Int:   i,
			Valid: valid,
		},
		NullContent: isNull,
	}
}

type Int8 struct {
	null.Int8
	NullContent bool
}

// NullInt8 creates a new Int8 with set null content
func NullInt8(s bool) Int8 {
	return NewInt8(0, true, s)
}

// Int8From creates a new Int8 that will never be blank.
func Int8From(s int8) Int8 {
	return NewInt8(s, true, false)
}

// Int8FromPtr creates a new Int8 that be null if s is nil.
func Int8FromPtr(s *int8) Int8 {
	if s == nil {
		return NewInt8(0, false, false)
	}
	return NewInt8(*s, true, false)
}

// NewInt8 creates a new Int8
func NewInt8(i int8, valid bool, isNull bool) Int8 {
	return Int8{
		Int8: null.Int8{
			Int8:  i,
			Valid: valid,
		},
		NullContent: isNull,
	}
}

type Int16 struct {
	null.Int16
	NullContent bool
}

// NullInt16 creates a new Int16 with set null content
func NullInt16(s bool) Int16 {
	return NewInt16(0, true, s)
}

// Int16From creates a new Int16 that will never be blank.
func Int16From(s int16) Int16 {
	return NewInt16(s, true, false)
}

// Int16FromPtr creates a new Int16 that be null if s is nil.
func Int16FromPtr(s *int16) Int16 {
	if s == nil {
		return NewInt16(0, false, false)
	}
	return NewInt16(*s, true, false)
}

// NewInt16 creates a new Int16
func NewInt16(i int16, valid bool, isNull bool) Int16 {
	return Int16{
		Int16: null.Int16{
			Int16: i,
			Valid: valid,
		},
		NullContent: isNull,
	}
}

type Int32 struct {
	null.Int32
	NullContent bool
}

// NullInt32 creates a new Int32 with set null content
func NullInt32(s bool) Int32 {
	return NewInt32(0, true, s)
}

// Int32From creates a new Int32 that will never be blank.
func Int32From(s int32) Int32 {
	return NewInt32(s, true, false)
}

// Int32FromPtr creates a new Int32 that be null if s is nil.
func Int32FromPtr(s *int32) Int32 {
	if s == nil {
		return NewInt32(0, false, false)
	}
	return NewInt32(*s, true, false)
}

// NewInt32 creates a new Int32
func NewInt32(i int32, valid bool, isNull bool) Int32 {
	return Int32{
		Int32: null.Int32{
			Int32: i,
			Valid: valid,
		},
		NullContent: isNull,
	}
}

type Int64 struct {
	null.Int64
	NullContent bool
}

// NullInt64 creates a new Int64 with set null content
func NullInt64(s bool) Int64 {
	return NewInt64(0, true, s)
}

// Int64From creates a new Int64 that will never be blank.
func Int64From(s int64) Int64 {
	return NewInt64(s, true, false)
}

// Int64FromPtr creates a new Int64 that be null if s is nil.
func Int64FromPtr(s *int64) Int64 {
	if s == nil {
		return NewInt64(0, false, false)
	}
	return NewInt64(*s, true, false)
}

// NewInt64 creates a new Int64
func NewInt64(i int64, valid bool, isNull bool) Int64 {
	return Int64{
		Int64: null.Int64{
			Int64: i,
			Valid: valid,
		},
		NullContent: isNull,
	}
}

type Uint struct {
	null.Uint
	NullContent bool
}

// NullUint creates a new Uint with set null content
func NullUint(s bool) Uint {
	return NewUint(0, true, s)
}

// UintFrom creates a new Uint that will never be blank.
func UintFrom(s uint) Uint {
	return NewUint(s, true, false)
}

// UintFromPtr creates a new Uint that be null if s is nil.
func UintFromPtr(s *uint) Uint {
	if s == nil {
		return NewUint(0, false, false)
	}
	return NewUint(*s, true, false)
}

// NewUint creates a new Uint
func NewUint(i uint, valid bool, isNull bool) Uint {
	return Uint{
		Uint: null.Uint{
			Uint:  i,
			Valid: valid,
		},
		NullContent: isNull,
	}
}

type Uint8 struct {
	null.Uint8
	NullContent bool
}

// NullUint8 creates a new Uint8 with set null content
func NullUint8(s bool) Uint8 {
	return NewUint8(0, true, s)
}

// Uint8From creates a new Uint8 that will never be blank.
func Uint8From(s uint8) Uint8 {
	return NewUint8(s, true, false)
}

// Uint8FromPtr creates a new Uint8 that be null if s is nil.
func Uint8FromPtr(s *uint8) Uint8 {
	if s == nil {
		return NewUint8(0, false, false)
	}
	return NewUint8(*s, true, false)
}

// NewUint8 creates a new Uint8
func NewUint8(i uint8, valid bool, isNull bool) Uint8 {
	return Uint8{
		Uint8: null.Uint8{
			Uint8: i,
			Valid: valid,
		},
		NullContent: isNull,
	}
}

type Uint16 struct {
	null.Uint16
	NullContent bool
}

// NullUint16 creates a new Uint16 with set null content
func NullUint16(s bool) Uint16 {
	return NewUint16(0, true, s)
}

// Uint16From creates a new Uint16 that will never be blank.
func Uint16From(s uint16) Uint16 {
	return NewUint16(s, true, false)
}

// Uint16FromPtr creates a new Uint16 that be null if s is nil.
func Uint16FromPtr(s *uint16) Uint16 {
	if s == nil {
		return NewUint16(0, false, false)
	}
	return NewUint16(*s, true, false)
}

// NewUint16 creates a new Uint16
func NewUint16(i uint16, valid bool, isNull bool) Uint16 {
	return Uint16{
		Uint16: null.Uint16{
			Uint16: i,
			Valid:  valid,
		},
		NullContent: isNull,
	}
}

type Uint32 struct {
	null.Uint32
	NullContent bool
}

// NullUint32 creates a new Uint32 with set null content
func NullUint32(s bool) Uint32 {
	return NewUint32(0, true, s)
}

// Uint32From creates a new Uint32 that will never be blank.
func Uint32From(s uint32) Uint32 {
	return NewUint32(s, true, false)
}

// Uint32FromPtr creates a new Uint32 that be null if s is nil.
func Uint32FromPtr(s *uint32) Uint32 {
	if s == nil {
		return NewUint32(0, false, false)
	}
	return NewUint32(*s, true, false)
}

// NewUint32 creates a new Uint32
func NewUint32(i uint32, valid bool, isNull bool) Uint32 {
	return Uint32{
		Uint32: null.Uint32{
			Uint32: i,
			Valid:  valid,
		},
		NullContent: isNull,
	}
}

type Uint64 struct {
	null.Uint64
	NullContent bool
}

// NullUint64 creates a new Uint64 with set null content
func NullUint64(s bool) Uint64 {
	return NewUint64(0, true, s)
}

// Uint64From creates a new Uint64 that will never be blank.
func Uint64From(s uint64) Uint64 {
	return NewUint64(s, true, false)
}

// Uint64FromPtr creates a new Uint64 that be null if s is nil.
func Uint64FromPtr(s *uint64) Uint64 {
	if s == nil {
		return NewUint64(0, false, false)
	}
	return NewUint64(*s, true, false)
}

// NewUint64 creates a new Uint64
func NewUint64(i uint64, valid bool, isNull bool) Uint64 {
	return Uint64{
		Uint64: null.Uint64{
			Uint64: i,
			Valid:  valid,
		},
		NullContent: isNull,
	}
}

type Float32 struct {
	null.Float32
	NullContent bool
}

// NullFloat32 creates a new Float32 with set null content
func NullFloat32(s bool) Float32 {
	return NewFloat32(0, true, s)
}

// Float32From creates a new Float32 that will never be blank.
func Float32From(s float32) Float32 {
	return NewFloat32(s, true, false)
}

// Float32FromPtr creates a new Float32 that be null if s is nil.
func Float32FromPtr(s *float32) Float32 {
	if s == nil {
		return NewFloat32(0, false, false)
	}
	return NewFloat32(*s, true, false)
}

// NewFloat32 creates a new Float32
func NewFloat32(i float32, valid bool, isNull bool) Float32 {
	return Float32{
		Float32: null.Float32{
			Float32: i,
			Valid:   valid,
		},
		NullContent: isNull,
	}
}

type Float64 struct {
	null.Float64
	NullContent bool
}

// NullFloat64 creates a new Float64 with set null content
func NullFloat64(s bool) Float64 {
	return NewFloat64(0, true, s)
}

// Float64From creates a new Float64 that will never be blank.
func Float64From(s float64) Float64 {
	return NewFloat64(s, true, false)
}

// Float64FromPtr creates a new Float64 that be null if s is nil.
func Float64FromPtr(s *float64) Float64 {
	if s == nil {
		return NewFloat64(0, false, false)
	}
	return NewFloat64(*s, true, false)
}

// NewFloat64 creates a new Float64
func NewFloat64(i float64, valid bool, isNull bool) Float64 {
	return Float64{
		Float64: null.Float64{
			Float64: i,
			Valid:   valid,
		},
		NullContent: isNull,
	}
}

type Time struct {
	null.Time
	NullContent bool
}

// NullTime creates a new Time with set null content
func NullTime(s bool) Time {
	return NewTime(time.Time{}, true, s)
}

// TimeFrom creates a new Time that will never be blank.
func TimeFrom(s time.Time) Time {
	return NewTime(s, true, false)
}

// TimeFromPtr creates a new Time that be null if s is nil.
func TimeFromPtr(s *time.Time) Time {
	if s == nil {
		return NewTime(time.Time{}, false, false)
	}
	return NewTime(*s, true, false)
}

// NewTime creates a new Time
func NewTime(i time.Time, valid bool, isNull bool) Time {
	return Time{
		Time: null.Time{
			Time:  i,
			Valid: valid,
		},
		NullContent: isNull,
	}
}

type Bool struct {
	null.Bool
	NullContent bool
}

// NullBool creates a new Bool with set null content
func NullBool(s bool) Bool {
	return NewBool(s, true, s)
}

// BoolFrom creates a new Bool that will never be blank.
func BoolFrom(s bool) Bool {
	return NewBool(s, true, false)
}

// BoolFromPtr creates a new Bool that be null if s is nil.
func BoolFromPtr(s *bool) Bool {
	if s == nil {
		return NewBool(false, false, false)
	}
	return NewBool(*s, true, false)
}

// NewBool creates a new Bool
func NewBool(i bool, valid bool, isNull bool) Bool {
	return Bool{
		Bool: null.Bool{
			Bool:  i,
			Valid: valid,
		},
		NullContent: isNull,
	}
}
