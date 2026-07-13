package context

import (
	"context"
	"strconv"
)

type requestBodyKey struct{}
type queryParamsKey struct{}
type pathParamsKey struct{}

func SetRequestBody(ctx context.Context, val any) context.Context {
	return context.WithValue(ctx, requestBodyKey{}, val)
}

func RequestBody[T any](ctx context.Context) *T {
	val, _ := ctx.Value(requestBodyKey{}).(*T)
	return val
}

func SetQueryParams(ctx context.Context, val any) context.Context {
	return context.WithValue(ctx, queryParamsKey{}, val)
}

func QueryParams[T any](ctx context.Context) *T {
	val, _ := ctx.Value(queryParamsKey{}).(*T)
	return val
}

func SetPathParams(ctx context.Context, val map[string]string) context.Context {
	return context.WithValue(ctx, pathParamsKey{}, val)
}

func PathParams(ctx context.Context) map[string]string {
	val, _ := ctx.Value(pathParamsKey{}).(map[string]string)
	return val
}

func PathParam[T any](ctx context.Context, name string) T {
	params := PathParams(ctx)
	if params == nil {
		var zero T
		return zero
	}
	raw, ok := params[name]
	if !ok {
		var zero T
		return zero
	}
	return convertParam[T](raw)
}

func convertParam[T any](raw string) T {
	var result any
	switch any(new(T)).(type) {
	case *int:
		v, _ := strconv.Atoi(raw)
		result = v
	case *int8:
		v, _ := strconv.ParseInt(raw, 10, 8)
		result = int8(v)
	case *int16:
		v, _ := strconv.ParseInt(raw, 10, 16)
		result = int16(v)
	case *int32:
		v, _ := strconv.ParseInt(raw, 10, 32)
		result = int32(v)
	case *int64:
		v, _ := strconv.ParseInt(raw, 10, 64)
		result = v
	case *uint:
		v, _ := strconv.ParseUint(raw, 10, 0)
		result = uint(v)
	case *uint8:
		v, _ := strconv.ParseUint(raw, 10, 8)
		result = uint8(v)
	case *uint16:
		v, _ := strconv.ParseUint(raw, 10, 16)
		result = uint16(v)
	case *uint32:
		v, _ := strconv.ParseUint(raw, 10, 32)
		result = uint32(v)
	case *uint64:
		v, _ := strconv.ParseUint(raw, 10, 64)
		result = v
	case *float32:
		v, _ := strconv.ParseFloat(raw, 32)
		result = float32(v)
	case *float64:
		v, _ := strconv.ParseFloat(raw, 64)
		result = v
	case *bool:
		v, _ := strconv.ParseBool(raw)
		result = v
	default:
		result = raw
	}
	return result.(T)
}
