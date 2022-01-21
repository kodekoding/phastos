package log

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetContext(t *testing.T) {
	testSet(t)
	testInit(t)
}

func testSet(t *testing.T) {
	ctx := context.Background()
	ctx = SetCtxRequestID(ctx, "request_test")
	ctx = SetCtxID(ctx, "context_test")
	require.Equal(t, "request_test", GetCtxRequestID(ctx))
	require.Equal(t, "context_test", GetCtxID(ctx))

}

func testInit(t *testing.T) {
	ctx := context.Background()
	ctx = InitLogContext(ctx)
	require.NotEqual(t, "", GetCtxRequestID(ctx))
}
