# Changelog

## [Unreleased] — OpenTelemetry Integration

### Added

#### `go/monitoring/otel.go` — New file
- `OTelConfig` struct for SDK configuration
- `InitOTelSDK()` — initializes OTel tracer provider with OTLP HTTP exporter
- `IsOTelActive()` — runtime flag to check if OTel SDK is initialized
- `StartSpan(ctx, name, attrs...)` — thin helper that creates a span and puts it in context, designed as migration companion for existing `StartSegment` patterns
- `OTelHTTPMiddleware()` — HTTP middleware that:
  - Extracts/injects W3C Trace Context (`traceparent` header)
  - Creates a span per request with HTTP attributes (method, URL, host, user-agent)
  - Sets `X-Request-Id` header to OTel trace ID (replaces manual `GenerateFastID()`)
  - Sets `traceresponse` header for downstream propagation

#### `go/api/app.go`
- `WithOTel()` option — initializes OTel SDK from env vars, registers OTel HTTP middleware as outermost wrapper
- `otelHandlerWrapper` struct — implements `Wrapper` interface for OTel middleware
- `app.otelTp *sdktrace.TracerProvider` field — stores tracer provider for graceful shutdown
- Graceful shutdown of OTel SDK via `defer` in `Start()`
- OTel span in `handleResponseError()` async goroutine — child span with `request_id` and `request_path` attributes

#### `go/database/definition.go`
- `isOTel bool` field on `SQL` struct

#### `go/database/sql.go`
- OTel spans in `Read()` — creates `PhastosDB-Read` span with `db.system`, `db.statement`, `db.params` attributes
- OTel spans in `Write()` — creates `PhastosDB-Write` span with same database attributes + `db.operation`
- OTel spans in `GenerateAddOnQuery()` — creates `PhastosDB-GeneratingAddOnQuery` span
- All DB spans coexist with existing New Relic segments (gated by `isNR`/`isOTel` flags)

#### `go/context/async.go`
- OTel span context propagation in `CreateAsyncContext()` — copies recording span from parent context to async context (alongside existing NR propagation)

#### `go/response/response.go`
- `ErrorChecking()` now uses OTel trace ID from `trace.SpanFromContext(ctx)` when available, falling back to `GenerateUUIDV4()`

### Changed

- `go.mod` / `go.sum` — added direct dependency on OpenTelemetry packages (`otel`, `otel/sdk`, `otel/exporters/otlp/otlptrace/otlptracehttp`)

### Backward Compatibility

All New Relic code is **unchanged and fully operational**. New OTel code is additive:
- `api.WithNewRelic()` continues to work exactly as before
- `api.WithOTel()` is a new separate option
- Both can coexist (NR segments + OTel spans in same request)
- Zero impact on existing deployments
