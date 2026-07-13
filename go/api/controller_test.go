package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRoute(t *testing.T) {
	dummyHandler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("ok")
	}

	t.Run("should create route with defaults", func(t *testing.T) {
		route := NewRoute(http.MethodGet, dummyHandler)
		assert.Equal(t, http.MethodGet, route.Method)
		assert.Equal(t, "", route.Path)
		assert.Equal(t, 1, route.Version)
		assert.Nil(t, route.Middlewares)
	})

	t.Run("should apply WithPath option", func(t *testing.T) {
		route := NewRoute(http.MethodPost, dummyHandler, WithPath("/users"))
		assert.Equal(t, http.MethodPost, route.Method)
		assert.Equal(t, "/users", route.Path)
	})

	t.Run("should apply WithVersion option", func(t *testing.T) {
		route := NewRoute(http.MethodGet, dummyHandler, WithVersion(2))
		assert.Equal(t, 2, route.Version)
	})

	t.Run("should apply WithMiddleware option", func(t *testing.T) {
		mw := func(next http.Handler) http.Handler { return next }
		route := NewRoute(http.MethodGet, dummyHandler, WithMiddleware(mw))
		assert.NotNil(t, route.Middlewares)
		assert.Len(t, *route.Middlewares, 1)
	})

	t.Run("should apply multiple options", func(t *testing.T) {
		route := NewRoute(http.MethodPut, dummyHandler,
			WithPath("/accounts/{id}"),
			WithVersion(3),
		)
		assert.Equal(t, http.MethodPut, route.Method)
		assert.Equal(t, "/accounts/{id}", route.Path)
		assert.Equal(t, 3, route.Version)
	})
}

func TestRoute_GetVersionedPath(t *testing.T) {
	dummyHandler := func(req Request, ctx context.Context) *Response { return nil }

	t.Run("should generate versioned path v1", func(t *testing.T) {
		route := NewRoute(http.MethodGet, dummyHandler, WithPath("/list"))
		result := route.GetVersionedPath("/users")
		assert.Equal(t, "/v1/users/list", result)
	})

	t.Run("should generate versioned path v2", func(t *testing.T) {
		route := NewRoute(http.MethodGet, dummyHandler, WithPath("/detail"), WithVersion(2))
		result := route.GetVersionedPath("/accounts")
		assert.Equal(t, "/v2/accounts/detail", result)
	})

	t.Run("should handle empty route path", func(t *testing.T) {
		route := NewRoute(http.MethodGet, dummyHandler)
		result := route.GetVersionedPath("/users")
		assert.Equal(t, "/v1/users", result)
	})

	t.Run("should handle empty controller path", func(t *testing.T) {
		route := NewRoute(http.MethodGet, dummyHandler, WithPath("/list"))
		result := route.GetVersionedPath("")
		assert.Equal(t, "/v1/list", result)
	})
}

func TestControllerImpl_UseMiddleware(t *testing.T) {
	t.Run("should return nil when no middlewares registered", func(t *testing.T) {
		ctrl := &ControllerImpl{}
		mw := ctrl.UseMiddleware("auth")
		assert.Nil(t, mw)
	})

	t.Run("should return nil for unknown key", func(t *testing.T) {
		ctrl := &ControllerImpl{
			registeredMiddlewares: map[string]any{},
		}
		mw := ctrl.UseMiddleware("nonexistent")
		assert.Nil(t, mw)
	})

	t.Run("should return middleware for valid key", func(t *testing.T) {
		expectedMw := func(next http.Handler) http.Handler { return next }
		ctrl := &ControllerImpl{
			registeredMiddlewares: map[string]any{
				"auth": expectedMw,
			},
		}
		mw := ctrl.UseMiddleware("auth")
		assert.NotNil(t, mw)
	})

	t.Run("should return nil when value is not a middleware func", func(t *testing.T) {
		ctrl := &ControllerImpl{
			registeredMiddlewares: map[string]any{
				"auth": "not-a-function",
			},
		}
		mw := ctrl.UseMiddleware("auth")
		assert.Nil(t, mw)
	})
}

func TestControllerImpl_JoinMiddleware(t *testing.T) {
	ctrl := &ControllerImpl{}
	mw1 := func(next http.Handler) http.Handler { return next }
	mw2 := func(next http.Handler) http.Handler { return next }

	result := ctrl.JoinMiddleware(mw1, mw2)
	assert.NotNil(t, result)
	assert.Len(t, *result, 2)
}

func TestParsePathParamTypes_AllString(t *testing.T) {
	types := parsePathParamTypes("/absence/{id}/recap/{month}")
	assert.Len(t, types, 2)
	assert.Equal(t, ParamString, types[0])
	assert.Equal(t, ParamString, types[1])
}

func TestParsePathParamTypes_TypedParams(t *testing.T) {
	types := parsePathParamTypes("/absence/{id:int64}/recap/{month:int}")
	assert.Len(t, types, 2)
	assert.Equal(t, ParamInt64, types[0])
	assert.Equal(t, ParamInt, types[1])
}

func TestParsePathParamTypes_Mixed(t *testing.T) {
	types := parsePathParamTypes("/{id:int64}/detail/{active:bool}")
	assert.Len(t, types, 2)
	assert.Equal(t, ParamInt64, types[0])
	assert.Equal(t, ParamBool, types[1])
}

func TestParsePathParamTypes_NoParams(t *testing.T) {
	types := parsePathParamTypes("/absence")
	assert.Len(t, types, 0)
}

func TestParsePathParamTypes_DefaultString(t *testing.T) {
	types := parsePathParamTypes("/{id}")
	assert.Len(t, types, 1)
	assert.Equal(t, ParamString, types[0])
}

func TestParsePathParamTypes_AllNumericTypes(t *testing.T) {
	types := parsePathParamTypes("/{a:int8}/{b:int16}/{c:int32}/{d:int64}/{e:uint8}/{f:uint16}/{g:uint32}/{h:uint64}/{i:float32}/{j:float64}")
	assert.Len(t, types, 10)
	assert.Equal(t, ParamInt8, types[0])
	assert.Equal(t, ParamInt16, types[1])
	assert.Equal(t, ParamInt32, types[2])
	assert.Equal(t, ParamInt64, types[3])
	assert.Equal(t, ParamUint8, types[4])
	assert.Equal(t, ParamUint16, types[5])
	assert.Equal(t, ParamUint32, types[6])
	assert.Equal(t, ParamUint64, types[7])
	assert.Equal(t, ParamFloat32, types[8])
	assert.Equal(t, ParamFloat64, types[9])
}

func TestParsePathParamTypes_UnknownTypeDefaultsToString(t *testing.T) {
	types := parsePathParamTypes("/{id:unknown}")
	assert.Len(t, types, 1)
	assert.Equal(t, ParamString, types[0])
}

func TestParsePathParamTypes_UintAndInt(t *testing.T) {
	types := parsePathParamTypes("/{id:uint}/{count:int}")
	assert.Len(t, types, 2)
	assert.Equal(t, ParamUint, types[0])
	assert.Equal(t, ParamInt, types[1])
}

func TestParsePathParamTypes_SingleParamWithType(t *testing.T) {
	types := parsePathParamTypes("/items/{id:int64}")
	assert.Len(t, types, 1)
	assert.Equal(t, ParamInt64, types[0])
}

func TestParsePathParamTypes_EmptyPath(t *testing.T) {
	types := parsePathParamTypes("")
	assert.Len(t, types, 0)
}

func TestParsePathParamTypes_UnclosedBrace(t *testing.T) {
	types := parsePathParamTypes("/{id")
	assert.Len(t, types, 0)
}

func TestPathParamType_String(t *testing.T) {
	assert.Equal(t, "string", ParamString.String())
	assert.Equal(t, "int", ParamInt.String())
	assert.Equal(t, "int8", ParamInt8.String())
	assert.Equal(t, "int16", ParamInt16.String())
	assert.Equal(t, "int32", ParamInt32.String())
	assert.Equal(t, "int64", ParamInt64.String())
	assert.Equal(t, "uint", ParamUint.String())
	assert.Equal(t, "uint8", ParamUint8.String())
	assert.Equal(t, "uint16", ParamUint16.String())
	assert.Equal(t, "uint32", ParamUint32.String())
	assert.Equal(t, "uint64", ParamUint64.String())
	assert.Equal(t, "float32", ParamFloat32.String())
	assert.Equal(t, "float64", ParamFloat64.String())
	assert.Equal(t, "bool", ParamBool.String())
}

func TestWithPath_SetsPathParamTypes(t *testing.T) {
	dummyHandler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("ok")
	}

	t.Run("typed params", func(t *testing.T) {
		route := NewRoute(http.MethodGet, dummyHandler, WithPath("/absence/{id:int64}/recap/{month}"))
		assert.Equal(t, "/absence/{id:int64}/recap/{month}", route.Path)
		assert.Len(t, route.PathParamTypes, 2)
		assert.Equal(t, ParamInt64, route.PathParamTypes[0])
		assert.Equal(t, ParamString, route.PathParamTypes[1])
	})

	t.Run("no params", func(t *testing.T) {
		route := NewRoute(http.MethodGet, dummyHandler, WithPath("/users"))
		assert.Len(t, route.PathParamTypes, 0)
	})

	t.Run("all string params", func(t *testing.T) {
		route := NewRoute(http.MethodGet, dummyHandler, WithPath("/{a}/{b}"))
		assert.Len(t, route.PathParamTypes, 2)
		assert.Equal(t, ParamString, route.PathParamTypes[0])
		assert.Equal(t, ParamString, route.PathParamTypes[1])
	})
}

func TestParsePathParamType_AllCases(t *testing.T) {
	assert.Equal(t, ParamInt, parsePathParamType("int"))
	assert.Equal(t, ParamInt8, parsePathParamType("int8"))
	assert.Equal(t, ParamInt16, parsePathParamType("int16"))
	assert.Equal(t, ParamInt32, parsePathParamType("int32"))
	assert.Equal(t, ParamInt64, parsePathParamType("int64"))
	assert.Equal(t, ParamUint, parsePathParamType("uint"))
	assert.Equal(t, ParamUint8, parsePathParamType("uint8"))
	assert.Equal(t, ParamUint16, parsePathParamType("uint16"))
	assert.Equal(t, ParamUint32, parsePathParamType("uint32"))
	assert.Equal(t, ParamUint64, parsePathParamType("uint64"))
	assert.Equal(t, ParamFloat32, parsePathParamType("float32"))
	assert.Equal(t, ParamFloat64, parsePathParamType("float64"))
	assert.Equal(t, ParamBool, parsePathParamType("bool"))
	assert.Equal(t, ParamString, parsePathParamType(""))
	assert.Equal(t, ParamString, parsePathParamType("string"))
	assert.Equal(t, ParamString, parsePathParamType("unknown_type"))
}
