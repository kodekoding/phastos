package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

var isOTelInit bool

func IsOTelActive() bool {
	return isOTelInit
}

type OTelConfig struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
}

func InitOTelSDK(ctx context.Context, cfg OTelConfig) (*sdktrace.TracerProvider, error) {
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}

	res := resource.NewWithAttributes(
		"https://opentelemetry.io/schemas/1.26.0",
		attribute.String("service.name", cfg.ServiceName),
		attribute.String("service.version", cfg.ServiceVersion),
		attribute.String("deployment.environment", cfg.Environment),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(5*time.Second)),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	isOTelInit = true

	SetProvider(&otelProvider{tp: tp})
	return tp, nil
}

func initOTelFromEnv() {
	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = os.Getenv("APP_NAME")
	}
	if serviceName == "" {
		return
	}
	cfg := OTelConfig{
		ServiceName:    serviceName,
		ServiceVersion: os.Getenv("APP_VERSION"),
		Environment:    os.Getenv("APP_ENV"),
	}
	if cfg.Environment == "" {
		cfg.Environment = "production"
	}
	_, _ = InitOTelSDK(context.Background(), cfg)
}

type otelProvider struct {
	tp *sdktrace.TracerProvider
}

type otelSpan struct {
	span trace.Span
}

func (p *otelProvider) StartSpan(ctx context.Context, name string) (context.Context, Span) {
	ctx, s := otel.Tracer("phastos").Start(ctx, name)
	return ctx, &otelSpan{span: s}
}

func (s *otelSpan) End() {
	s.span.End()
}

func (s *otelSpan) SetAttributes(kv ...attribute.KeyValue) {
	s.span.SetAttributes(kv...)
}

func (p *otelProvider) GetTraceId(ctx context.Context) string {
	if sc := trace.SpanFromContext(ctx).SpanContext(); sc.HasTraceID() {
		return sc.TraceID().String()
	}
	return ""
}

func (p *otelProvider) GetLogLink(traceId string) string {
	uiURL := os.Getenv("OTEL_UI_URL")
	if uiURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/trace/%s", strings.TrimRight(uiURL, "/"), traceId)
}

func OTelHTTPMiddleware(serviceName string) func(http.Handler) http.Handler {
	propagator := otel.GetTextMapPropagator()
	tracer := otel.Tracer(serviceName)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			spanName := r.Method + " " + r.URL.Path
			ctx, span := tracer.Start(ctx, spanName,
				trace.WithAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.url", r.URL.String()),
					attribute.String("http.host", r.Host),
					attribute.String("http.user_agent", r.UserAgent()),
				),
			)
			defer span.End()

			if sc := span.SpanContext(); sc.HasTraceID() {
				r.Header.Set("X-Request-Id", sc.TraceID().String())
			}

			propagator.Inject(ctx, propagation.HeaderCarrier(w.Header()))

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
