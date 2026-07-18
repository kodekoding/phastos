# Monitoring (Observability)

Phastos provides a unified monitoring abstraction that supports **OpenTelemetry**, **New Relic**, or **both simultaneously** via a composite provider pattern. Global functions auto-route to the active provider, so application code never needs to know which backend is in use.

---

## Provider Architecture

### Provider Interface

```go
// go/monitoring/monitor.go:15

type Provider interface {
    StartSpan(ctx context.Context, name string) (context.Context, Span)
    GetTraceId(ctx context.Context) string
    GetLogLink(traceId string) string
}

type Span interface {
    End()
    SetAttributes(kv ...attribute.KeyValue)
}
```

### Active Provider Lifecycle

1. **Default**: `noopProvider` — safe no-op that returns empty context and no-op spans.
2. **SetProvider(p)** — directly assign one active provider.
3. **SetProviders(providers...)** — wraps 0 providers as noop, 1 as a direct provider, 2+ as a `compositeProvider`.
4. **compositeProvider** fans out every call to all registered backends.

```
SetProviders(nil)         → noopProvider{}
SetProviders(otel)        → otelProvider{}
SetProviders(nr, otel)    → compositeProvider{providers: [nr, otel]}
```

---

## Global Functions

All application code calls these package-level functions. They delegate to `activeProvider` internally.

```go
// go/monitoring/monitor.go:52

ctx, span := monitoring.StartSpan(ctx, "my-operation")
defer span.End()

traceId := monitoring.GetTraceId(ctx)

logLink := monitoring.GetLogLink(traceId)
```

| Function | Returns | Description |
|---|---|---|
| `StartSpan(ctx, name)` | `(context.Context, Span)` | Creates a span. Always call `defer span.End()`. |
| `GetTraceId(ctx)` | `string` | Trace/transaction ID from active provider. Empty if no monitoring active. |
| `GetLogLink(traceId)` | `string` | Clickable log-dashboard URL. Empty if provider not configured with a UI URL. |
| `ActiveProvider()` | `Provider` | Returns the currently active provider (useful for introspection). |

### NoopProvider (Default)

When no monitoring provider is configured, every call is a safe no-op:
- `StartSpan` returns the original context and a `noopSpan` whose `End()` does nothing.
- `GetTraceId` and `GetLogLink` return empty strings.
- Callers fall back to UUID generation (e.g., `helper.GenerateUUIDV4()` in `response.go:154`).

### StartSpan in Practice

Phastos internal packages trace operations automatically:

| Package | File | Example |
|---|---|---|
| `database` | `sql.go:109` | `monitoring.StartSpan(ctx, "PhastosDB-Read")` |
| `database` | `sql.go:606` | `monitoring.StartSpan(ctx, "PhastosDB-Write")` |
| `cache` | `redis.go:232` | `monitoring.StartSpan(ctx, `Redis-${cmd}`)` |

### GetTraceId in Practice

- `go/response/response.go:152` — error responses include the trace ID for correlation.
- Falls back to `helper.GenerateUUIDV4()` when no provider is active.

### GetLogLink in Practice

- `go/notifications/slack/slack.go:106` — error notifications include a "View Logs" link when the provider supports it.

---

## OpenTelemetry

### Configuration and Initialization

```go
// go/monitoring/otel.go:26

type OTelConfig struct {
    ServiceName    string
    ServiceVersion string
    Environment    string
}
```

#### Manual Init

```go
tp, err := monitoring.InitOTelSDK(ctx, monitoring.OTelConfig{
    ServiceName:    "my-service",
    ServiceVersion: "1.0.0",
    Environment:    "staging",
})
if err != nil {
    log.Fatal(err)
}
defer tp.Shutdown(ctx)
```

`InitOTelSDK` both creates the tracer provider AND calls `SetProvider()`, making it the active provider.

#### Auto-Init from Environment

```go
// go/monitoring/monitor.go:42

func Init() {
    providerName := os.Getenv("MONITORING_PROVIDER")
    switch providerName {
    case "otel":
        initOTelFromEnv()
    default:
        InitNewRelic()
    }
}
```

When `MONITORING_PROVIDER=otel`, the SDK auto-configures from these environment variables:

| Env Variable | Fallback | Default |
|---|---|---|
| `OTEL_SERVICE_NAME` | `APP_NAME` | — (skips init if both empty) |
| `APP_VERSION` | — | — |
| `APP_ENV` | — | `production` |

### Trace Export

- Exporter: OTLP over HTTP (`otlptracehttp`).
- Batch span export every **5 seconds** (configurable via `sdktrace.WithBatchTimeout`).
- W3C Trace Context propagation (TraceContext + Baggage composite propagator).

### OpenTelemetry Provider Details

```go
// go/monitoring/otel.go:94

type otelProvider struct {
    tp *sdktrace.TracerProvider
}
```

| Method | Behavior |
|---|---|
| `StartSpan(ctx, name)` | Creates a span via `otel.Tracer("phastos").Start(ctx, name)` |
| `GetTraceId(ctx)` | Extracts `SpanContext.TraceID().String()` from the current span |
| `GetLogLink(traceId)` | Reads `OTEL_UI_URL` from env, returns `{url}/trace/{traceId}` |

### OTel HTTP Middleware

```go
// go/monitoring/otel.go:130

func OTelHTTPMiddleware(serviceName string) func(http.Handler) http.Handler
```

- Extracts W3C trace context from incoming request headers.
- Creates a span for each HTTP request named `METHOD /path`.
- Sets HTTP attributes: `http.method`, `http.url`, `http.host`, `http.user_agent`.
- Injects trace context into response headers.
- Sets `X-Request-Id` header to the trace ID.

### Required Environment Variables for Log Link

| Variable | Purpose |
|---|---|
| `OTEL_UI_URL` | Base URL for the OTel UI (e.g., Jaeger or Grafana). If unset, `GetLogLink` returns `""`. |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP collector endpoint (auto-read by `otlptracehttp.New(ctx)`). |

---

## New Relic

### Configuration and Initialization

```go
// go/monitoring/new_relic.go:29,37

func InitNewRelic(opts ...NewRelicOpts) *newRelic

func InitNewRelicOnly(opts ...NewRelicOpts) (*newRelic, Provider)
```

- `InitNewRelic` — creates the New Relic app AND calls `SetProvider()`, making it the active provider.
- `InitNewRelicOnly` — creates the app but does NOT call `SetProvider()`. Used internally by `WithNewRelic()` so the composite resolution happens later in `NewApp()`.

#### Options

```go
monitoring.WithAppName("my-app")
monitoring.WithLicenseKey("0123456789012345678901234567890123456789")
```

#### Environment Variables

| Variable | Fallback | Required |
|---|---|---|
| `NEW_RELIC_APP_NAME` | `APP_NAME` | Yes |
| `NEW_RELIC_LICENSE_KEY` | — | Yes |
| `NEW_RELIC_ACCOUNT_ID` | — | Only for `GetLogLink` (log dashboard link) |

If either app name or license key is empty, the New Relic agent will fail to start (fatal log).

#### Default Config

- `AppLogDecoratingEnabled(true)`
- `AppLogForwardingEnabled(true)`
- `CodeLevelMetricsEnabled(true)`
- Ignores error status codes: `400, 401, 403, 404, 405, 422, 429`

### New Relic Provider Details

```go
// go/monitoring/new_relic.go:107

type nrProvider struct {
    app *newrelic.Application
}
```

| Method | Behavior |
|---|---|
| `StartSpan(ctx, name)` | Creates a `newrelic.Segment` from the transaction in context. Falls back to noop if no transaction exists. |
| `GetTraceId(ctx)` | Returns `txn.GetLinkingMetadata().TraceID`. Empty if no transaction in context. |
| `GetLogLink(traceId)` | Builds a New Relic One logger query URL: `https://one.newrelic.com/logger/query?accountId={id}&nrql=SELECT * FROM Log WHERE trace.id = '{traceId}'` |

### New Relic HTTP Middleware

```go
// go/monitoring/new_relic.go:158

func NewRelicHTTPMiddleware(app *newrelic.Application) func(http.Handler) http.Handler
```

- Starts a New Relic transaction per HTTP request.
- Sets `X-Request-Id` header to the trace ID.
- Injects `newrelic.Transaction` into the request context (available via `newrelic.FromContext(ctx)`).
- Gracefully passes through if `app` is nil.

#### Helper Functions

```go
func BeginTrxFromContext(ctx context.Context) *newrelic.Transaction
func NewContext(parentCtx context.Context, txn *newrelic.Transaction) context.Context
```

---

## Composite Provider (Both New Relic + OpenTelemetry)

```go
// go/monitoring/composite.go:9

type compositeProvider struct {
    providers []Provider
}
```

When both `WithNewRelic()` and `WithOTel()` are passed to `NewApp()`, both providers are collected and wrapped in a `compositeProvider`:

```go
// go/api/app.go:139
var monitoringProviders []monitoring.Provider
if apiApp.nrProv != nil {
    monitoringProviders = append(monitoringProviders, apiApp.nrProv)
}
if apiApp.otelProv != nil {
    monitoringProviders = append(monitoringProviders, apiApp.otelProv)
}
monitoring.SetProviders(monitoringProviders...)
```

### Fan-Out Behavior

| Method | Behavior |
|---|---|
| `StartSpan` | Fans out to **all** providers sequentially. Each provider's returned context is fed into the next. Returns a `compositeSpan` that delegates `End()` and `SetAttributes()` to all child spans. |
| `GetTraceId` | Returns the **first non-empty** trace ID from any provider. |
| `GetLogLink` | Returns the **first non-empty** log link from any provider. |

```go
type compositeSpan struct {
    spans []Span
}

func (s *compositeSpan) End() {
    for _, sp := range s.spans {
        sp.End()
    }
}

func (s *compositeSpan) SetAttributes(kv ...attribute.KeyValue) {
    for _, sp := range s.spans {
        sp.SetAttributes(kv...)
    }
}
```

---

## API Integration

### App Options

```go
// go/api/app.go:278,290

func WithNewRelic() Options
func WithOTel() Options
```

Both are `Options` for `api.NewApp()`.

#### WithNewRelic()

1. Calls `monitoring.InitNewRelicOnly()` (does NOT call `SetProvider`).
2. Stores `*newrelic.Application` in `app.newRelic`.
3. Stores the NR provider in `app.nrProv`.
4. Wraps every HTTP handler with `NewRelicHTTPMiddleware` via `WrapToApp`.

#### WithOTel()

1. Reads `OTEL_SERVICE_NAME` (fallback: `APP_NAME`).
2. Calls `monitoring.InitOTelOnly()` (does NOT call `SetProvider`).
3. Stores `*sdktrace.TracerProvider` in `app.otelTp`.
4. Stores the OTel provider in `app.otelProv`.
5. Wraps every HTTP handler with `OTelHTTPMiddleware` via `WrapToApp`.

### Composite Resolution in NewApp()

After all options are applied, `NewApp()` checks which providers were registered and calls `SetProviders()`:

```go
// go/api/app.go:139-146
var monitoringProviders []monitoring.Provider
if apiApp.nrProv != nil {
    monitoringProviders = append(monitoringProviders, apiApp.nrProv)
}
if apiApp.otelProv != nil {
    monitoringProviders = append(monitoringProviders, apiApp.otelProv)
}
monitoring.SetProviders(monitoringProviders...)
```

This means:
- `WithNewRelic()` alone → `SetProviders(nrProv)` → New Relic only.
- `WithOTel()` alone → `SetProviders(otelProv)` → OpenTelemetry only.
- `WithNewRelic()` + `WithOTel()` → `SetProviders(nrProv, otelProv)` → Composite (both).

---

## Quick Reference — Environment Variables

| Variable | Provider | Purpose |
|---|---|---|
| `MONITORING_PROVIDER` | All | Set to `"otel"` to auto-init OTel via `Init()`, defaults to New Relic |
| `OTEL_SERVICE_NAME` | OTel | Service name (fallback: `APP_NAME`) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTel | OTLP collector endpoint |
| `OTEL_UI_URL` | OTel | Log/trace UI base URL for `GetLogLink` |
| `APP_NAME` | Both | Fallback for service name |
| `APP_VERSION` | Both | Application version |
| `APP_ENV` | OTel | Deployment environment (default: `production`) |
| `NEW_RELIC_APP_NAME` | NR | Application name (fallback: `APP_NAME`) |
| `NEW_RELIC_LICENSE_KEY` | NR | License key |
| `NEW_RELIC_ACCOUNT_ID` | NR | Account ID for log link generation |

### Usage Examples

```go
// Option A: New Relic only
app := api.NewApp(
    api.WithNewRelic(),
    api.WithAppPort(8080),
)

// Option B: OpenTelemetry only
app := api.NewApp(
    api.WithOTel(),
    api.WithAppPort(8080),
)

// Option C: Both simultaneously
app := api.NewApp(
    api.WithNewRelic(),
    api.WithOTel(),
    api.WithAppPort(8080),
)

// Option D: Manual provider (no api.App)
monitoring.InitNewRelic()
// or
monitoring.InitOTelSDK(ctx, monitoring.OTelConfig{...})

// Then in any handler or service:
ctx, span := monitoring.StartSpan(ctx, "process-order")
defer span.End()

span.SetAttributes(attribute.String("order.id", orderID))

traceId := monitoring.GetTraceId(ctx)
logLink := monitoring.GetLogLink(traceId)
```

---

## How the Packages Connect

```
NewApp()
  ├── WithNewRelic() → InitNewRelicOnly() → stores nrProv
  ├── WithOTel()     → InitOTelOnly()     → stores otelProv
  └── SetProviders(nrProv, otelProv, ...) → resolves composite

HTTP Request
  ├── NewRelicHTTPMiddleware / OTelHTTPMiddleware
  │     creates span, sets X-Request-Id, injects context
  ├── Handler calls monitoring.StartSpan(ctx, name)
  │     fans out to all active providers
  ├── response.NewAPIResponse() → monitoring.GetTraceId(ctx)
  │     fallback: helper.GenerateUUIDV4()
  └── notifications/slack → monitoring.GetLogLink(traceId)
        adds "View Logs" link when available
```
