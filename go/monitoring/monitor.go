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
}

var activeProvider Provider = &noopProvider{}

func SetProvider(p Provider) {
	activeProvider = p
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

type noopSpan struct{}

func (noopSpan) End() {}

func (noopSpan) SetAttributes(...attribute.KeyValue) {}

type noopProvider struct{}

func (noopProvider) StartSpan(ctx context.Context, name string) (context.Context, Span) {
	return ctx, noopSpan{}
}
