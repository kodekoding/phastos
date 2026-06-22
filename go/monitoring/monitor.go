package monitoring

import (
	"context"
	"os"

	"go.opentelemetry.io/otel/attribute"
)

type Span interface {
	End()
	SetAttributes(kv ...attribute.KeyValue)
}

type Provider interface {
	StartSpan(ctx context.Context, name string) (context.Context, Span)
	GetTraceId(ctx context.Context) string
	GetLogLink(traceId string) string
}

var activeProvider Provider = &noopProvider{}

func SetProvider(p Provider) {
	activeProvider = p
}

func SetProviders(providers ...Provider) {
	switch len(providers) {
	case 0:
		SetProvider(&noopProvider{})
	case 1:
		SetProvider(providers[0])
	default:
		SetProvider(&compositeProvider{providers: providers})
	}
}

func ActiveProvider() Provider {
	return activeProvider
}

func Init() {
	providerName := os.Getenv("MONITORING_PROVIDER")
	switch providerName {
	case "otel":
		initOTelFromEnv()
	default:
		InitNewRelic()
	}
}

func StartSpan(ctx context.Context, name string) (context.Context, Span) {
	return activeProvider.StartSpan(ctx, name)
}

func GetTraceId(ctx context.Context) string {
	return activeProvider.GetTraceId(ctx)
}

func GetLogLink(traceId string) string {
	return activeProvider.GetLogLink(traceId)
}

type noopSpan struct{}

func (noopSpan) End() {}

func (noopSpan) SetAttributes(...attribute.KeyValue) {}

type noopProvider struct{}

func (noopProvider) StartSpan(ctx context.Context, name string) (context.Context, Span) {
	return ctx, noopSpan{}
}

func (noopProvider) GetTraceId(ctx context.Context) string {
	return ""
}

func (noopProvider) GetLogLink(traceId string) string {
	return ""
}
