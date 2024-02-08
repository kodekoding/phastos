package null

// extended version from "github.com/volatiletech/null" to set null content
import "github.com/volatiletech/null"

type String struct {
	null.String
	NullContent bool
}

type Int struct {
	null.Int
	NullContent bool
}

type Int8 struct {
	null.Int8
	NullContent bool
}

type Int16 struct {
	null.Int16
	NullContent bool
}

type Int32 struct {
	null.Int32
	NullContent bool
}

type Int64 struct {
	null.Int64
	NullContent bool
}

type Uint struct {
	null.Uint
	NullContent bool
}

type Uint8 struct {
	null.Uint8
	NullContent bool
}

type Uint16 struct {
	null.Uint16
	NullContent bool
}

type Uint32 struct {
	null.Uint32
	NullContent bool
}

type Uint64 struct {
	null.Uint64
	NullContent bool
}

type Float32 struct {
	null.Float32
	NullContent bool
}

type Float64 struct {
	null.Float64
	NullContent bool
}

type Time struct {
	null.Time
	NullContent bool
}

type Bool struct {
	null.Bool
	NullContent bool
}
