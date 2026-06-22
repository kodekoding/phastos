package monitoring

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/newrelic/go-agent/v3/newrelic"
	"go.opentelemetry.io/otel/attribute"
)

var newNewApplication = newrelic.NewApplication
var logFatalln = log.Fatalln

type (
	NewRelicOpts func(relics *newRelic)
	NewRelics    interface {
	}

	newRelic struct {
		appName    string
		licenseKey string
		app        *newrelic.Application
	}
)

func InitNewRelic(opts ...NewRelicOpts) *newRelic {
	nr, prov := initNewRelic(opts...)
	if prov != nil {
		SetProvider(prov)
	}
	return nr
}

func InitNewRelicOnly(opts ...NewRelicOpts) (*newRelic, Provider) {
	return initNewRelic(opts...)
}

func initNewRelic(opts ...NewRelicOpts) (*newRelic, Provider) {
	var newRelicPlatform = newRelic{
		appName:    os.Getenv("NEW_RELIC_APP_NAME"),
		licenseKey: os.Getenv("NEW_RELIC_LICENSE_KEY"),
	}

	if newRelicPlatform.appName == "" {
		newRelicPlatform.appName = os.Getenv("APP_NAME")
	}

	for _, opt := range opts {
		opt(&newRelicPlatform)
	}

	app, err := newNewApplication(
		newrelic.ConfigAppName(newRelicPlatform.appName),
		newrelic.ConfigLicense(newRelicPlatform.licenseKey),
		newrelic.ConfigAppLogDecoratingEnabled(true),
		newrelic.ConfigAppLogForwardingEnabled(true),
		newrelic.ConfigCodeLevelMetricsEnabled(true),
		func(config *newrelic.Config) {
			config.ErrorCollector.IgnoreStatusCodes = []int{
				http.StatusForbidden,
				http.StatusUnprocessableEntity,
				http.StatusUnauthorized,
				http.StatusNotFound,
				http.StatusMethodNotAllowed,
				http.StatusBadRequest,
				http.StatusTooManyRequests,
			}
		},
	)
	newRelicPlatform.app = app
	if err != nil {
		//nolint:gosec // G706: operational fatal log includes controlled error output for startup diagnostics
		logFatalln("Failed to connect new relic: ", err.Error())
		return nil, nil
	}

	return &newRelicPlatform, &nrProvider{app: app}
}

func (n *newRelic) GetApp() *newrelic.Application {
	return n.app
}

func BeginTrxFromContext(ctx context.Context) *newrelic.Transaction {
	return newrelic.FromContext(ctx)
}

func NewContext(parentCtx context.Context, txn *newrelic.Transaction) context.Context {
	return newrelic.NewContext(parentCtx, txn)
}

func WithAppName(appName string) NewRelicOpts {
	return func(relics *newRelic) {
		relics.appName = appName
	}
}

func WithLicenseKey(licenseKey string) NewRelicOpts {
	return func(relics *newRelic) {
		relics.licenseKey = licenseKey
	}
}

type nrProvider struct {
	app *newrelic.Application
}

type nrSpan struct {
	segment *newrelic.Segment
	noop    bool
}

func (p *nrProvider) StartSpan(ctx context.Context, name string) (context.Context, Span) {
	txn := newrelic.FromContext(ctx)
	if txn == nil {
		return ctx, &nrSpan{noop: true}
	}
	seg := txn.StartSegment(name)
	return newrelic.NewContext(ctx, txn), &nrSpan{segment: seg}
}

func (s *nrSpan) End() {
	if !s.noop && s.segment != nil {
		s.segment.End()
	}
}

func (s *nrSpan) SetAttributes(kv ...attribute.KeyValue) {
	if s.noop || s.segment == nil {
		return
	}
	for _, attr := range kv {
		s.segment.AddAttribute(string(attr.Key), attr.Value.AsInterface())
	}
}

func (p *nrProvider) GetTraceId(ctx context.Context) string {
	if txn := newrelic.FromContext(ctx); txn != nil {
		metadata := txn.GetLinkingMetadata()
		return metadata.TraceID
	}
	return ""
}

func (p *nrProvider) GetLogLink(traceId string) string {
	if accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID"); accountID != "" {
		return fmt.Sprintf(
			"https://one.newrelic.com/logger/query?accountId=%s&nrql=SELECT * FROM Log WHERE trace.id = '%s'",
			accountID, traceId,
		)
	}
	return ""
}

func NewRelicHTTPMiddleware(app *newrelic.Application) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if app == nil {
				next.ServeHTTP(w, r)
				return
			}
			txn := app.StartTransaction(r.URL.Path)
			defer txn.End()

			metadata := txn.GetLinkingMetadata()
			if metadata.TraceID != "" {
				r.Header.Set("X-Request-Id", metadata.TraceID)
			}

			ctx := newrelic.NewContext(r.Context(), txn)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
