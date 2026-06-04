package monitoring

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
)

func TestNoopSpan_End(t *testing.T) {
	s := noopSpan{}
	s.End() // should not panic
}

func TestNoopSpan_SetAttributes(t *testing.T) {
	s := noopSpan{}
	s.SetAttributes(attribute.String("key", "value"))
}

func TestNoopProvider_StartSpan(t *testing.T) {
	p := noopProvider{}
	ctx := context.Background()
	newCtx, span := p.StartSpan(ctx, "test")
	assert.Equal(t, ctx, newCtx)
	assert.NotNil(t, span)
}

func TestDefaultProviderIsNoop(t *testing.T) {
	assert.IsType(t, &noopProvider{}, activeProvider)
}

func TestStartSpan_DefaultReturnsNoop(t *testing.T) {
	ctx := context.Background()
	_, span := StartSpan(ctx, "test")
	assert.NotNil(t, span)
	span.End()
	span.SetAttributes(attribute.String("k", "v"))
}

func TestStartSpan_ReturnsContext(t *testing.T) {
	ctx := context.Background()
	ctx2, span := StartSpan(ctx, "test")
	assert.NotNil(t, ctx2)
	assert.NotNil(t, span)
}

func TestSetProvider(t *testing.T) {
	orig := activeProvider
	defer func() { activeProvider = orig }()

	p := &noopProvider{}
	SetProvider(p)
	assert.Same(t, p, activeProvider)
}

func TestActiveProvider(t *testing.T) {
	orig := activeProvider
	defer func() { activeProvider = orig }()

	SetProvider(&noopProvider{})
	assert.IsType(t, &noopProvider{}, ActiveProvider())
}

func TestSpanInterface_NoopSatisfies(t *testing.T) {
	var _ Span = noopSpan{}
}

func TestProviderInterface_NoopSatisfies(t *testing.T) {
	var _ Provider = (*noopProvider)(nil)
}

// mockProvider is a test double for monitoring.Provider
type mockProvider struct {
	spans []string
}

func (m *mockProvider) StartSpan(ctx context.Context, name string) (context.Context, Span) {
	m.spans = append(m.spans, name)
	return ctx, noopSpan{}
}

func TestStartSpan_CallsProvider(t *testing.T) {
	orig := activeProvider
	defer func() { activeProvider = orig }()

	mock := &mockProvider{}
	SetProvider(mock)

	ctx := context.Background()
	_, s := StartSpan(ctx, "test-span")
	assert.NotNil(t, s)
	assert.Equal(t, []string{"test-span"}, mock.spans)
}

func TestStartSpan_WithActiveProvider(t *testing.T) {
	orig := activeProvider
	defer func() { activeProvider = orig }()

	mock := &mockProvider{}
	SetProvider(mock)

	ctx := context.Background()
	ctx2, span := StartSpan(ctx, "test")
	assert.NotNil(t, ctx2)
	assert.NotNil(t, span)
}

func TestSetProvider_Nil(t *testing.T) {
	orig := activeProvider
	defer func() { activeProvider = orig }()

	SetProvider(nil)
	assert.Nil(t, activeProvider)
}
