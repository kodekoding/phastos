# Configuration Reference

## Environment Variables

| Variable | Default | Description | Used By |
|---|---|---|---|
| `APP_NAME` | — | Application name | log, monitoring, server, OTel fallback |
| `APPS_ENV` | `development` | Environment (`development`/`staging`/`production`/`local`) | env, server |
| `APP_ENV` | — | Environment for OpenTelemetry | monitoring (OTel) |
| `APP_VERSION` | — | Application version string | log, monitoring |
| `COMMIT_HASH` | — | Git commit hash | app startup |
| `CONTAINER_NAME` | — | Container identifier | log, server notifications |
| `DATABASE_ENGINE` | — | Database engine (`mysql` / `postgres`) | database |
| `DATABASE_CONN_STRING_MASTER` | — | Master DB connection string (DSN) | database |
| `DATABASE_CONN_STRING_FOLLOWER` | — | Follower DB connection string (DSN) | database |
| `DATABASE_MAX_OPEN_CONN` | `10` | Max open connections | database |
| `DATABASE_MAX_IDLE_CONN` | `2` | Max idle connections | database |
| `DATABASE_CONN_MAX_LIFETIME` | `300` | Max connection lifetime in seconds | database |
| `DATABASE_CONN_MAX_IDLE_TIME` | `45` | Max idle time in seconds | database |
| `DATABASE_SLOW_QUERY_THRESHOLD` | `1` | Slow query threshold in seconds | database |
| `DATABASE_SLOW_QUERY_WARNING` | `false` | Enable slow query warning (bool) | database |
| `REDIS_CONN_STRING` | — | Redis address (`host:port`) | cache, api/resources |
| `REDIS_PREFIX_KEY` | `phastos:` | Key prefix for cache store | cache |
| `REDIS_TIMEOUT` | — | Redis connection timeout in seconds | cache, api/resources |
| `REDIS_MAX_ACTIVE` | — | Max active connections | cache, api/resources |
| `REDIS_MAX_IDLE` | — | Max idle connections | cache, api/resources |
| `REDIS_MAX_RETRY` | — | Max retry attempts | cache, api/resources |
| `REDIS_DB` | — | Redis database number | cache, api/resources |
| `REDIS_PASSWORD` | — | Redis password | cache, api/resources |
| `REDIS_USERNAME` | — | Redis username (Redis 6+) | cache, api/resources |
| `JWT_SIGNING_KEY` | — | HMAC-SHA256 signing key | middleware, helper/jwt |
| `JWT_ISSUER` | `phastos` | JWT issuer claim | helper/jwt |
| `SERVICE_SECRET` | — | Static auth secret for middleware | middleware |
| `MONITORING_PROVIDER` | — | Set `otel` for OpenTelemetry; unset defaults to New Relic | monitoring |
| `NEW_RELIC_APP_NAME` | — (falls back to `APP_NAME`) | New Relic application name | monitoring |
| `NEW_RELIC_LICENSE_KEY` | — | New Relic license key | monitoring |
| `NEW_RELIC_ACCOUNT_ID` | — | New Relic account ID for log links | monitoring |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | — | OTLP collector HTTP endpoint | monitoring, log |
| `OTEL_SERVICE_NAME` | — (falls back to `APP_NAME`) | OpenTelemetry service name | monitoring |
| `OTEL_UI_URL` | — | OTel UI base URL for log links | monitoring |
| `CORS_ORIGIN` | — | Comma-separated allowed origins | api |
| `CORS_HEADER` | — | Comma-separated additional allowed headers | api |
| `PPROF_ENABLED` | — | Enable pprof profiling (overrides `WithPprof`) | api |
| `SINGLEFLIGHT_ACTIVE` | `false` | Enable request deduplication (bool) | api |
| `CRON_JOB_TIMEOUT_PROCESS` | `1` | Cron job timeout in minutes | cron |
| `NOTIFICATIONS_SLACK_WEBHOOK_URL` | — | Slack webhook URL for error notifications | notifications, cron |
| `NOTIFICATION_SLACK_INFO_WEBHOOK` | — | Slack webhook URL for info notifications | notifications, cron, importer |
| `NOTIFICATIONS_TELEGRAM_TOKEN` | — | Telegram bot token | notifications |
| `NOTIFY_SERVICE_STATUS` | `false` | Enable service start/stop Slack notifications (bool) | api/server |
| `DRIVE_CREDENTIALS_PATH` | — | Google Drive credentials file path | storage |
| `STORAGE_CREDENTIALS_PATH` | — | Google Cloud Storage credentials file path | storage |
| `GOOGLE_APPLICATION_CREDENTIALS` | — | Fallback GCP credentials file path | storage |
| `GCS_ACCESS_TOKEN` | — | Google Cloud Storage access token | storage |

---

## Options Functions

### api — `api.Options`

| Function | Description |
|---|---|
| `WithAppPort(port int)` | Set HTTP server port (default `8000`) |
| `ReadTimeout(readTimeout int)` | Set server read timeout in seconds (default `3`) |
| `WriteTimeout(writeTimeout int)` | Set server write timeout in seconds (default `3`) |
| `WithAPITimeout(apiTimeout int)` | Set handler timeout in seconds; `0` = synchronous (default `3`) |
| `WithTimezone(timezone string)` | Set timezone for date/time helpers (default `"Asia/Jakarta"`) |
| `WithCronJob(timezone ...string)` | Enable cron scheduler with optional timezone (default `"Asia/Jakarta"`) |
| `WithPprof(enabled bool)` | Enable pprof profiling at `/debug/pprof/` (default `true`) |
| `WithSSE()` | Enable Server-Sent Events hub at `/events` |
| `WithFastHttp()` | Use fasthttp router instead of chi |
| `WithSkipLogPaths(paths ...string)` | Skip request logging for given paths (`/ping` always skipped) |
| `WithOpenAPI()` | Enable OpenAPI 3.0.3 spec at `/docs/openapi.json` and Swagger UI at `/docs` |
| `WithGlobalMiddleware(handlers ...func(http.Handler) http.Handler)` | Register global middlewares applied to all endpoints |
| `WithNewRelic()` | Enable New Relic APM tracing |
| `WithOTel()` | Enable OpenTelemetry tracing |

### cache — `cache.Options`

| Function | Description |
|---|---|
| `WithAddress(address string)` | Redis server address (`host:port`) |
| `WithDatabaseNo(dbNo int)` | Redis database number |
| `WithTimeout(timeout int)` | Connection, read, and write timeout in seconds |
| `WithMaxActive(maxActive int)` | Max active connections in pool (default `MaxIdle * 5`) |
| `WithMaxIdle(maxIdle int)` | Max idle connections in pool (default `10`) |
| `WithMaxRetry(maxRetry int)` | Retry count for Redis operations (default `10`) |
| `WithPassword(password string)` | Redis password |
| `WithUsername(username string)` | Redis username (Redis 6+ ACL) |

### monitoring

| Function | Description |
|---|---|
| `InitNewRelic(opts ...NewRelicOpts) *newRelic` | Initialize New Relic APM, registers as active provider |
| `monitoring.WithAppName(appName string)` | New Relic option: set application name |
| `monitoring.WithLicenseKey(licenseKey string)` | New Relic option: set license key |
| `InitOTelSDK(ctx context.Context, cfg OTelConfig) (*sdktrace.TracerProvider, error)` | Initialize OpenTelemetry SDK, registers as active provider |

### cron — `cron.Options`

| Function | Description |
|---|---|
| `WithTimeZone(timeZone string)` | Set timezone for cron scheduler |

### log — `log.LoggerOption`

| Function | Description |
|---|---|
| `WithNewRelicApp(app *newrelic.Application)` | Set New Relic application for log forwarding |
| `WithAppVersion(appVersion string)` | Set application version in log fields |
| `WithAppPort(appPort int)` | Set application port in log fields |
| `WithOTelLogEndpoint()` | Configure TCP log writer to OTel collector (auto-derived from `OTEL_EXPORTER_OTLP_ENDPOINT`) |
