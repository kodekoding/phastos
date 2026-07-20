# Phastos README + Documentation Design

**Goal:** Rewrite the root `README.md` and create comprehensive per-module documentation under `docs/` to make Phastos publishable and easy for external Go engineers to adopt.

**Approach:** `README.md` as landing page (badges, quick-start, full app example, TOC), detailed modules in `docs/*.md`.

## File Structure

| File | Responsibility |
|------|---------------|
| `README.md` | Landing page: badges, one-liner, installation, quick-start (15-min app), feature highlights, TOC links |
| `docs/api.md` | Full HTTP framework: App lifecycle, controllers, routes, groups, Handler/HandlerV2, OpenAPI, middleware, monitoring opts |
| `docs/database.md` | SQL: Connect, Read/Write, QueryOpts, TableRequest, CUDResponse, transactions, pagination, CRUD helpers |
| `docs/cache.md` | Redis: New, Get/Set, HGet/HSet, fallback with singleflight, streams, options |
| `docs/monitoring.md` | OTEL + New Relic, composite provider, Init, StartSpan, GetTraceId, GetLogLink, env vars |
| `docs/integrations.md` | Cron scheduler, SSE events, Notifications (Slack/Telegram/FCM), Mail (Mandrill/SMTP), Slack socket mode |
| `docs/utilities.md` | Generator (PDF/CSV/Excel/QR/Banner), Storage (GCS), Importer, Auth middleware (JWT + static), env, log |
| `docs/configuration.md` | All .env keys, all Options (App, Cache, DB, Monitoring, Cron, Notifications) |

## Content Outline

### README.md

```
# Phastos — Go Development Kit
Badges: Go version, version tag, license, CI, Go Report, pkg.go.dev

## Overview (3-4 sentences)
Phastos is a batteries-included Go framework for building production-ready
HTTP APIs. It bundles chi/fasthttp routing, SQL query builder, Redis caching
with fallback, OpenTelemetry/NewRelic monitoring, auto-generated OpenAPI docs,
cron jobs, SSE events, file generators, and multi-platform notifications —
all wired together with sensible defaults.

## Installation
go get github.com/kodekoding/phastos/v2

## Quick Start — 15-Minute App
Full example: main.go that demonstrates:
  1. Init App with port, timeout, monitoring, OpenAPI
  2. DB connection + Read/Write
  3. Cache Get with fallback
  4. Controller + routes
  5. Start server

## Key Features
List: HTTP Framework, SQL Query Builder, Redis Cache w/ Fallback,
OpenTelemetry + NewRelic, OpenAPI Auto-Gen, Cron Scheduler,
File Generators, Notifications, more...

## Documentation
Table linking to docs/*.md

## License
MIT
```

### docs/api.md

```
# API Framework

## Creating an App
- api.NewApp(opts...) — all Options listed
- app.Init() — what it does (router, cron, DB, wrappers)
- app.Start() — graceful shutdown

## Controllers & Routes
- Controller interface, ControllerConfig
- Route struct: Method, Path, Handler, Version, Middlewares, SubRoutes
- NewRoute() vs NewGroup()
- Path param types ({id:int64})
- Handler (Request, context.Context) *Response vs HandlerV2 func(context.Context) (any, error)
- HandlerV2 with auto-binding (RouteDoc annotations)

## Middleware
- Global: WithGlobalMiddleware, AddGlobalMiddleware
- Per-controller: ControllerConfig.Middlewares
- Per-route: Route.Middlewares, JoinMiddleware
- Built-in: JWTAuth, StaticAuth, RateLimiter
- Custom: RegisterMiddlewareFunc + UseMiddleware

## Response
- NewResponse(), ReleaseResponse()
- SetData, SetMessage, SetError, SetHTTPError, SetFile
- Error types: BadRequest, InternalServerError, etc.

## OpenAPI / Swagger
- WithOpenAPI() option
- RouteDoc annotations: Summary, Description, Tags, RequestType, ResponseType, ErrorResponses
- Serves /docs and /docs/openapi.json

## Server Config
- Port, timeouts, timezone, pprof, SSL

## Testing Handlers
- app.wrapHandler(handler) → http.HandlerFunc
- httptest usage
```

### docs/database.md

```
# Database / Query Builder

## Connection
- database.Connect() + env vars

## Read Operations
- QueryOpts, TableRequest (pagination, search, ordering, date range)
- Read(ctx, opts) — single or list
- SelectResponse + ResponseMetaData

## Write Operations
- CUDConstructData, CUDResponse
- Write(ctx, opts) — Insert, Update, Delete, Bulk, Upsert, SoftDelete

## Transactions
- Begin() / Finish()
- Using trx in QueryOpts.Trx

## CRUD Helpers
- ReadRepo / WriteRepo interfaces
- UsecaseCRUD

## Slow Query Notifications
- Config threshold + Slack notification
```

### docs/cache.md

```
# Redis Cache

## Connection
- cache.New(opts...)
- Options: Address, DatabaseNo, Timeout, MaxActive, MaxIdle, MaxRetry, Password, Username

## Basic Operations
- Get(ctx, key, &dest) — auto JSON unmarshal
- Set(ctx, key, value, expire)
- Del(ctx, key)
- HGet/HSet/HDel, HGetAll, HSetBulk

## Fallback Pattern
- FallbackFn type
- Get/HGet with fallback: cache miss → call DB → cache it → return
- Singleflight deduplication for concurrent cache misses

## Redis Streams
- PublishStream, SubscribeStream
- XGroupCreateMkStream, XReadGroup, XAck

## Context Integration
- WrapToHandler, WrapToContext, GetCacheFromContext
```

### docs/monitoring.md

```
# Monitoring (Observability)

## Provider Architecture
- Provider interface: StartSpan, GetTraceId, GetLogLink
- Global functions route to active provider

## OpenTelemetry
- InitOTelSDK, InitOTelOnly
- OTelHTTPMiddleware
- Env vars: OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_SERVICE_NAME, etc.

## New Relic
- InitNewRelic, InitNewRelicOnly
- NewRelicHTTPMiddleware
- Env vars: NEW_RELIC_APP_NAME, NEW_RELIC_LICENSE_KEY

## Composite (Both)
- SetProviders() — fan-out spans to all providers
- WithNewRelic() + WithOTel() in NewApp

## Getting Trace IDs
- monitoring.GetTraceId(ctx)
- monitoring.GetLogLink(traceId)
```

### docs/integrations.md

```
# Integrations

## Cron Scheduler
- App.AddScheduler(pattern, handler)
- cron.New(), RegisterScheduler, Start/Stop
- Slack notifications on job lifecycle

## SSE (Server-Sent Events)
- WithSSE() option
- Hub: Broadcast, client registration, missed messages
- GET /events, GET /events/missed-msg

## Notifications
- notifications.New(ActivateSlack, ActivateTelegram, ActivateFirebase)
- WrapToHandler / WrapToContext
- Auto 500 error notification (Slack + Telegram via context)

## Mail (Mandrill / SMTP)
- Mandrills interface, SMTP
- Templates, attachments, merge vars

## Slack Socket Mode
- slack.NewSlackApp, WithSocketMode, WithHttp
- Event handlers for slash commands, interactions, events
```

### docs/utilities.md

```
# Utilities

## File Generators
- PDF (wkhtmltopdf), CSV, Excel (excelize), QR Code, Banner Image
- FileGenerator interface

## Cloud Storage (GCS)
- Upload/Download, Signed URLs, bucket management

## Data Importer
- CSV/Excel import with worker pools, transactions

## Auth Middleware
- JWTAuth: Bearer token, HS256, claims in context
- StaticAuth: secret header

## Environment & Logging
- env.ServiceEnv(), IsProduction() etc.
- log.Get() with NewRelic/OTel writers

## Helper Utilities
- JWT generation, AES crypto, Slack notification, UUID, string helpers
```

### docs/configuration.md

```
# Configuration Reference

## Environment Variables
Table of all env vars: name, default, description, used by

## App Options
All With* functions for NewApp

## Cache Options
All With* functions for cache.New

## Database Env Vars
DATABASE_ENGINE, DATABASE_CONN_STRING_*, pool config

## Monitoring Env Vars
OTEL_*, NEW_RELIC_*, MONITORING_PROVIDER
```

## Content Style

- Every code example MUST be syntactically correct Go that compiles
- Every function signature MUST match actual source code
- Package imports shown explicitly
- Comments in English
- Examples prefer minimal but complete snippets (imports + main when needed)

## Constraints

- Do NOT modify any .go file — documentation only
- Replace root `README.md` entirely
- Remove `go/README.md` (merge its content into new docs)
- Follow GitHub-flavored markdown
- Code blocks tagged ```go
- Use relative links between docs files
