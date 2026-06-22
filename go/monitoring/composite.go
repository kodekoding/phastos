package monitoring

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
)

type compositeProvider struct {
	providers []Provider
}

func (p *compositeProvider) StartSpan(ctx context.Context, name string) (context.Context, Span) {
	var spans []Span
	var lastCtx context.Context = ctx
	for _, prov := range p.providers {
		c, sp := prov.StartSpan(lastCtx, name)
		lastCtx = c
		spans = append(spans, sp)
	}
	return lastCtx, &compositeSpan{spans: spans}
}

func (p *compositeProvider) GetTraceId(ctx context.Context) string {
	for _, prov := range p.providers {
		if id := prov.GetTraceId(ctx); id != "" {
			return id
		}
	}
	return ""
}

func (p *compositeProvider) GetLogLink(traceId string) string {
	for _, prov := range p.providers {
		if link := prov.GetLogLink(traceId); link != "" {
			return link
		}
	}
	return ""
}

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
