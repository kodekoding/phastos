package context

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testPayload struct {
	Name string
	Age  int
}

func TestSetAndGetRequestBody(t *testing.T) {
	t.Run("should set and get request body from context", func(t *testing.T) {
		ctx := context.Background()
		payload := &testPayload{Name: "test", Age: 30}
		ctx = SetRequestBody(ctx, payload)

		result := RequestBody[testPayload](ctx)
		assert.NotNil(t, result)
		assert.Equal(t, "test", result.Name)
		assert.Equal(t, 30, result.Age)
	})

	t.Run("should overwrite request body", func(t *testing.T) {
		ctx := context.Background()
		first := &testPayload{Name: "first", Age: 1}
		second := &testPayload{Name: "second", Age: 2}

		ctx = SetRequestBody(ctx, first)
		ctx = SetRequestBody(ctx, second)

		result := RequestBody[testPayload](ctx)
		assert.NotNil(t, result)
		assert.Equal(t, "second", result.Name)
		assert.Equal(t, 2, result.Age)
	})
}

func TestRequestBody_NilWhenNotSet(t *testing.T) {
	t.Run("should return nil when request body not set", func(t *testing.T) {
		result := RequestBody[testPayload](context.Background())
		assert.Nil(t, result)
	})
}

func TestRequestBody_WrongType(t *testing.T) {
	t.Run("should return nil when context value is wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), requestBodyKey{}, "not-a-pointer")
		result := RequestBody[testPayload](ctx)
		assert.Nil(t, result)
	})
}

func TestSetAndGetQueryParams(t *testing.T) {
	t.Run("should set and get query params from context", func(t *testing.T) {
		ctx := context.Background()
		params := &testPayload{Name: "query", Age: 25}
		ctx = SetQueryParams(ctx, params)

		result := QueryParams[testPayload](ctx)
		assert.NotNil(t, result)
		assert.Equal(t, "query", result.Name)
		assert.Equal(t, 25, result.Age)
	})

	t.Run("should overwrite query params", func(t *testing.T) {
		ctx := context.Background()
		first := &testPayload{Name: "first", Age: 1}
		second := &testPayload{Name: "second", Age: 2}

		ctx = SetQueryParams(ctx, first)
		ctx = SetQueryParams(ctx, second)

		result := QueryParams[testPayload](ctx)
		assert.NotNil(t, result)
		assert.Equal(t, "second", result.Name)
	})
}

func TestQueryParams_NilWhenNotSet(t *testing.T) {
	t.Run("should return nil when query params not set", func(t *testing.T) {
		result := QueryParams[testPayload](context.Background())
		assert.Nil(t, result)
	})
}

func TestQueryParams_WrongType(t *testing.T) {
	t.Run("should return nil when context value is wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), queryParamsKey{}, "not-a-pointer")
		result := QueryParams[testPayload](ctx)
		assert.Nil(t, result)
	})
}

func TestSetAndGetPathParams(t *testing.T) {
	t.Run("should set and get path params from context", func(t *testing.T) {
		ctx := context.Background()
		params := map[string]string{"id": "42", "name": "test"}
		ctx = SetPathParams(ctx, params)

		result := PathParams(ctx)
		assert.NotNil(t, result)
		assert.Equal(t, "42", result["id"])
		assert.Equal(t, "test", result["name"])
	})

	t.Run("should overwrite path params", func(t *testing.T) {
		ctx := context.Background()
		first := map[string]string{"a": "1"}
		second := map[string]string{"b": "2"}

		ctx = SetPathParams(ctx, first)
		ctx = SetPathParams(ctx, second)

		result := PathParams(ctx)
		assert.NotNil(t, result)
		assert.Equal(t, "2", result["b"])
		assert.Equal(t, "", result["a"])
	})
}

func TestPathParams_NilWhenNotSet(t *testing.T) {
	t.Run("should return nil when path params not set", func(t *testing.T) {
		result := PathParams(context.Background())
		assert.Nil(t, result)
	})
}

func TestPathParams_WrongType(t *testing.T) {
	t.Run("should return nil when context value is wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), pathParamsKey{}, "not-a-map")
		result := PathParams(ctx)
		assert.Nil(t, result)
	})
}

func TestPathParam_String(t *testing.T) {
	t.Run("should retrieve string path param", func(t *testing.T) {
		ctx := context.Background()
		ctx = SetPathParams(ctx, map[string]string{"id": "42", "name": "test"})

		id := PathParam[string](ctx, "id")
		assert.Equal(t, "42", id)

		name := PathParam[string](ctx, "name")
		assert.Equal(t, "test", name)
	})
}

func TestPathParam_NotSet(t *testing.T) {
	t.Run("should return zero value when path param not set in context", func(t *testing.T) {
		ctx := context.Background()
		val := PathParam[string](ctx, "nonexistent")
		assert.Equal(t, "", val)

		ival := PathParam[int64](ctx, "nonexistent")
		assert.Equal(t, int64(0), ival)
	})
}

func TestPathParam_KeyNotFound(t *testing.T) {
	t.Run("should return zero value when key is not in path params", func(t *testing.T) {
		ctx := context.Background()
		ctx = SetPathParams(ctx, map[string]string{"a": "1"})

		val := PathParam[string](ctx, "nonexistent")
		assert.Equal(t, "", val)
	})
}

func TestConvertParam_Int(t *testing.T) {
	t.Run("should convert to int", func(t *testing.T) {
		assert.Equal(t, 42, convertParam[int]("42"))
		assert.Equal(t, -10, convertParam[int]("-10"))
		assert.Equal(t, 0, convertParam[int]("invalid"))
	})
}

func TestConvertParam_Int8(t *testing.T) {
	t.Run("should convert to int8", func(t *testing.T) {
		assert.Equal(t, int8(100), convertParam[int8]("100"))
		assert.Equal(t, int8(0), convertParam[int8]("invalid"))
	})
}

func TestConvertParam_Int16(t *testing.T) {
	t.Run("should convert to int16", func(t *testing.T) {
		assert.Equal(t, int16(1000), convertParam[int16]("1000"))
	})
}

func TestConvertParam_Int32(t *testing.T) {
	t.Run("should convert to int32", func(t *testing.T) {
		assert.Equal(t, int32(99999), convertParam[int32]("99999"))
	})
}

func TestConvertParam_Int64(t *testing.T) {
	t.Run("should convert to int64", func(t *testing.T) {
		assert.Equal(t, int64(42), convertParam[int64]("42"))
		assert.Equal(t, int64(0), convertParam[int64]("invalid"))
	})
}

func TestConvertParam_Uint(t *testing.T) {
	t.Run("should convert to uint", func(t *testing.T) {
		assert.Equal(t, uint(42), convertParam[uint]("42"))
		assert.Equal(t, uint(0), convertParam[uint]("invalid"))
	})
}

func TestConvertParam_Uint8(t *testing.T) {
	t.Run("should convert to uint8", func(t *testing.T) {
		assert.Equal(t, uint8(200), convertParam[uint8]("200"))
		assert.Equal(t, uint8(0), convertParam[uint8]("invalid"))
	})
}

func TestConvertParam_Uint16(t *testing.T) {
	t.Run("should convert to uint16", func(t *testing.T) {
		assert.Equal(t, uint16(5000), convertParam[uint16]("5000"))
	})
}

func TestConvertParam_Uint32(t *testing.T) {
	t.Run("should convert to uint32", func(t *testing.T) {
		assert.Equal(t, uint32(88888), convertParam[uint32]("88888"))
	})
}

func TestConvertParam_Uint64(t *testing.T) {
	t.Run("should convert to uint64", func(t *testing.T) {
		assert.Equal(t, uint64(99999), convertParam[uint64]("99999"))
		assert.Equal(t, uint64(0), convertParam[uint64]("invalid"))
	})
}

func TestConvertParam_Float32(t *testing.T) {
	t.Run("should convert to float32", func(t *testing.T) {
		assert.InDelta(t, float32(3.14), convertParam[float32]("3.14"), 0.001)
		assert.Equal(t, float32(0), convertParam[float32]("invalid"))
	})
}

func TestConvertParam_Float64(t *testing.T) {
	t.Run("should convert to float64", func(t *testing.T) {
		assert.InDelta(t, 3.14, convertParam[float64]("3.14"), 0.001)
		assert.Equal(t, float64(0), convertParam[float64]("invalid"))
	})
}

func TestConvertParam_Bool(t *testing.T) {
	t.Run("should convert to bool", func(t *testing.T) {
		assert.Equal(t, true, convertParam[bool]("true"))
		assert.Equal(t, true, convertParam[bool]("1"))
		assert.Equal(t, false, convertParam[bool]("false"))
		assert.Equal(t, false, convertParam[bool]("0"))
		assert.Equal(t, false, convertParam[bool]("invalid"))
	})
}

func TestConvertParam_Default(t *testing.T) {
	t.Run("should return raw string for unsupported types", func(t *testing.T) {
		assert.Equal(t, "hello", convertParam[string]("hello"))
	})
}

func TestPreserveContextValues(t *testing.T) {
	t.Run("request body should not interfere with path params", func(t *testing.T) {
		ctx := context.Background()
		ctx = SetRequestBody(ctx, &testPayload{Name: "req", Age: 10})
		ctx = SetPathParams(ctx, map[string]string{"id": "99"})

		reqBody := RequestBody[testPayload](ctx)
		assert.NotNil(t, reqBody)
		assert.Equal(t, "req", reqBody.Name)

		id := PathParam[string](ctx, "id")
		assert.Equal(t, "99", id)
	})
}
