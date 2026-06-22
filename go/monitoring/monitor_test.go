package monitoring

import (
	"context"
	"os"
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
	spans    []string
	traceId  string
	logLink  string
}

func (m *mockProvider) StartSpan(ctx context.Context, name string) (context.Context, Span) {
	m.spans = append(m.spans, name)
	return ctx, noopSpan{}
}

func (m *mockProvider) GetTraceId(ctx context.Context) string {
	return m.traceId
}

func (m *mockProvider) GetLogLink(traceId string) string {
	return m.logLink
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

func TestNoopProvider_GetTraceId(t *testing.T) {
	p := noopProvider{}
	ctx := context.Background()
	assert.Equal(t, "", p.GetTraceId(ctx))
}

func TestNoopProvider_GetLogLink(t *testing.T) {
	p := noopProvider{}
	assert.Equal(t, "", p.GetLogLink("some-trace-id"))
}

func TestGetTraceId_NoopProvider(t *testing.T) {
	orig := activeProvider
	defer func() { activeProvider = orig }()

	activeProvider = &noopProvider{}
	ctx := context.Background()
	assert.Equal(t, "", GetTraceId(ctx))
}

func TestGetLogLink_NoopProvider(t *testing.T) {
	orig := activeProvider
	defer func() { activeProvider = orig }()

	activeProvider = &noopProvider{}
	assert.Equal(t, "", GetLogLink("some-trace-id"))
}

func TestGetTraceId_MockProvider(t *testing.T) {
	orig := activeProvider
	defer func() { activeProvider = orig }()

	mock := &mockProvider{traceId: "test-trace-123"}
	SetProvider(mock)

	ctx := context.Background()
	traceId := GetTraceId(ctx)
	assert.Equal(t, "test-trace-123", traceId)
}

func TestGetLogLink_MockProvider(t *testing.T) {
	orig := activeProvider
	defer func() { activeProvider = orig }()

	mock := &mockProvider{logLink: "https://example.com/trace/test-trace-123"}
	SetProvider(mock)

	logLink := GetLogLink("test-trace-123")
	assert.Equal(t, "https://example.com/trace/test-trace-123", logLink)
}

func TestProviderInterface_NoopSatisfiesNewMethods(t *testing.T) {
	var _ Provider = (*noopProvider)(nil)
}

func TestProviderInterface_MockSatisfies(t *testing.T) {
	var _ Provider = (*mockProvider)(nil)
}

// --- composite provider tests ---

type trackSpan struct {
	ended    bool
	attrs    []attribute.KeyValue
}

func (s *trackSpan) End()              { s.ended = true }
func (s *trackSpan) SetAttributes(kv ...attribute.KeyValue) { s.attrs = append(s.attrs, kv...) }

type trackProvider struct {
	spans   []*trackSpan
	traceId string
	logLink string
}

func (p *trackProvider) StartSpan(ctx context.Context, name string) (context.Context, Span) {
	s := &trackSpan{}
	p.spans = append(p.spans, s)
	return ctx, s
}

func (p *trackProvider) GetTraceId(ctx context.Context) string   { return p.traceId }
func (p *trackProvider) GetLogLink(traceId string) string        { return p.logLink }

func TestSetProviders_Single(t *testing.T) {
	orig := activeProvider
	defer func() { activeProvider = orig }()

	mock := &mockProvider{}
	SetProviders(mock)
	assert.IsType(t, &mockProvider{}, activeProvider)
}

func TestSetProviders_Composite(t *testing.T) {
	orig := activeProvider
	defer func() { activeProvider = orig }()

	p1 := &trackProvider{}
	p2 := &trackProvider{}
	SetProviders(p1, p2)
	_, ok := activeProvider.(*compositeProvider)
	assert.True(t, ok, "expected compositeProvider when multiple providers given")
}

func TestSetProviders_None(t *testing.T) {
	orig := activeProvider
	defer func() { activeProvider = orig }()

	SetProviders()
	assert.IsType(t, &noopProvider{}, activeProvider)
}

func TestCompositeProvider_StartSpan_FanOut(t *testing.T) {
	orig := activeProvider
	defer func() { activeProvider = orig }()

	p1 := &trackProvider{}
	p2 := &trackProvider{}
	SetProviders(p1, p2)

	ctx := context.Background()
	_, span := StartSpan(ctx, "test-span")

	assert.Len(t, p1.spans, 1, "p1 should receive StartSpan call")
	assert.Len(t, p2.spans, 1, "p2 should receive StartSpan call")
	assert.NotNil(t, span)
}

func TestCompositeSpan_End_FanOut(t *testing.T) {
	p1 := &trackProvider{}
	p2 := &trackProvider{}
	cp := &compositeProvider{providers: []Provider{p1, p2}}

	_, span := cp.StartSpan(context.Background(), "test")
	span.End()

	assert.True(t, p1.spans[0].ended, "p1 span should be ended")
	assert.True(t, p2.spans[0].ended, "p2 span should be ended")
}

func TestCompositeSpan_SetAttributes_FanOut(t *testing.T) {
	p1 := &trackProvider{}
	p2 := &trackProvider{}
	cp := &compositeProvider{providers: []Provider{p1, p2}}

	_, span := cp.StartSpan(context.Background(), "test")
	span.SetAttributes(attribute.String("key", "val"))

	assert.Len(t, p1.spans[0].attrs, 1, "p1 span should have attributes")
	assert.Len(t, p2.spans[0].attrs, 1, "p2 span should have attributes")
}

func TestCompositeProvider_GetTraceId_FirstNonEmpty(t *testing.T) {
	p1 := &trackProvider{traceId: ""}
	p2 := &trackProvider{traceId: "otel-trace-123"}
	cp := &compositeProvider{providers: []Provider{p1, p2}}

	ctx := context.Background()
	id := cp.GetTraceId(ctx)
	assert.Equal(t, "otel-trace-123", id)
}

func TestCompositeProvider_GetTraceId_AllEmpty(t *testing.T) {
	p1 := &trackProvider{traceId: ""}
	p2 := &trackProvider{traceId: ""}
	cp := &compositeProvider{providers: []Provider{p1, p2}}

	ctx := context.Background()
	id := cp.GetTraceId(ctx)
	assert.Equal(t, "", id)
}

func TestCompositeProvider_GetLogLink_FirstNonEmpty(t *testing.T) {
	p1 := &trackProvider{logLink: ""}
	p2 := &trackProvider{logLink: "https://gitlab.com/trace/abc"}
	cp := &compositeProvider{providers: []Provider{p1, p2}}

	link := cp.GetLogLink("abc")
	assert.Equal(t, "https://gitlab.com/trace/abc", link)
}

func TestInitNewRelicOnly_ReturnsProvider(t *testing.T) {
	orig := activeProvider
	defer func() { SetProvider(orig) }()

	os.Setenv("NEW_RELIC_APP_NAME", "test")
	os.Setenv("NEW_RELIC_LICENSE_KEY", "0123456789012345678901234567890123456789")
	defer os.Unsetenv("NEW_RELIC_APP_NAME")
	defer os.Unsetenv("NEW_RELIC_LICENSE_KEY")

	SetProvider(&noopProvider{})

	_, prov := InitNewRelicOnly()
	assert.NotNil(t, prov, "InitNewRelicOnly should return a non-nil provider")
	assert.IsType(t, &noopProvider{}, activeProvider, "InitNewRelicOnly should NOT call SetProvider")
}
