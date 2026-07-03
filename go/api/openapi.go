package api

import (
	"reflect"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

var (
	// schemaTypeObject and schemaTypeString are reused across schema generation.
	schemaTypeObject = &openapi3.Types{"object"}
	schemaTypeString = &openapi3.Types{"string"}
)

// openapiHTML is the Swagger UI HTML page served at /docs.
// It loads Swagger UI from CDN and fetches the spec from /docs/openapi.json.
var openapiHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>API Docs</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: '/docs/openapi.json',
      dom_id: '#swagger-ui',
    })
  </script>
</body>
</html>`

// buildOpenAPISpec generates an OpenAPI 3.0.3 spec from all registered routes.
func (app *App) buildOpenAPISpec() *openapi3.T {
	spec := &openapi3.T{
		OpenAPI: "3.0.3",
		Info: &openapi3.Info{
			Title:   app.getAppName(),
			Version: appVersion,
		},
		Paths: openapi3.NewPaths(),
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{},
		},
	}

	for _, entry := range app.routeRegistry {
		pathItem := spec.Paths.Find(entry.Path)
		if pathItem == nil {
			pathItem = &openapi3.PathItem{}
			spec.Paths.Set(entry.Path, pathItem)
		}
		app.buildOperation(pathItem, entry)
	}

	// Add security scheme components
	for _, info := range app.middlewareDocs {
		if info.SecurityScheme != nil {
			scheme := &openapi3.SecurityScheme{
				Type: info.SecurityScheme.Type,
				Name: info.SecurityScheme.Name,
				In:   info.SecurityScheme.In,
			}
			spec.Components.SecuritySchemes[info.SecurityScheme.Name] = &openapi3.SecuritySchemeRef{
				Value: scheme,
			}
		}
	}

	return spec
}

func (app *App) buildOperation(item *openapi3.PathItem, entry routeRegistryEntry) {
	operation := &openapi3.Operation{
		Summary:     entry.Doc.Summary,
		Description: entry.Doc.Description,
		Tags:        entry.Doc.Tags,
	}

	// Request body
	if entry.Doc.RequestType != nil {
		schema := app.generateSchema(entry.Doc.RequestType)
		operation.RequestBody = &openapi3.RequestBodyRef{
			Value: openapi3.NewRequestBody().
				WithJSONSchema(schema).
				WithRequired(true),
		}
	}

	// Response
	if entry.Doc.ResponseType != nil {
		schema := app.generateSchema(entry.Doc.ResponseType)
		operation.AddResponse(200, openapi3.NewResponse().
			WithDescription("Success").
			WithJSONSchema(schema),
		)
	}

	// Error responses
	for _, errResp := range entry.Doc.ErrorResponses {
		operation.AddResponse(errResp.StatusCode, openapi3.NewResponse().
			WithDescription(errResp.Description),
		)
	}

	// Security
	if entry.Doc.Security != nil {
		operation.Security = &openapi3.SecurityRequirements{
			openapi3.SecurityRequirement{entry.Doc.Security.Name: {}},
		}
	}

	// Headers (global)
	if len(entry.Doc.Headers) > 0 {
		params := make([]*openapi3.ParameterRef, 0, len(entry.Doc.Headers))
		for _, h := range entry.Doc.Headers {
			params = append(params, &openapi3.ParameterRef{
				Value: &openapi3.Parameter{
					Name:        h.Name,
					In:          "header",
					Description: h.Description,
					Required:    h.Required,
					Schema: &openapi3.SchemaRef{
						Value: &openapi3.Schema{
							Type: &openapi3.Types{h.Type},
						},
					},
				},
			})
		}
		operation.Parameters = params
	}

	item.SetOperation(entry.Method, operation)
}

// generateSchema converts a Go struct to an OpenAPI Schema via reflection.
func (app *App) generateSchema(model any) *openapi3.Schema {
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return &openapi3.Schema{Type: schemaTypeObject}
	}

	schema := &openapi3.Schema{
		Type:       schemaTypeObject,
		Properties: openapi3.Schemas{},
	}

	for i := range t.NumField() {
		field := t.Field(i)

		// Skip unexported
		if !field.IsExported() {
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}

		name := strings.Split(jsonTag, ",")[0]
		if name == "" {
			name = field.Name
		}

		propSchema := app.fieldToSchema(field)
		schema.Properties[name] = &openapi3.SchemaRef{
			Value: propSchema,
		}

		// Check validate:"required"
		validateTag := field.Tag.Get("validate")
		if strings.Contains(validateTag, "required") {
			schema.Required = append(schema.Required, name)
		}
	}

	return schema
}

func (app *App) fieldToSchema(field reflect.StructField) *openapi3.Schema {
	t := field.Type

	// Dereference pointer
	nullable := false
	if t.Kind() == reflect.Ptr {
		nullable = true
		t = t.Elem()
	}

	// Special types
	if t == reflect.TypeOf(time.Time{}) {
		return &openapi3.Schema{
			Type:   schemaTypeString,
			Format: "date-time",
		}
	}

	switch t.Kind() {
	case reflect.String:
		return &openapi3.Schema{Type: schemaTypeString}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &openapi3.Schema{Type: &openapi3.Types{"integer"}}
	case reflect.Float32, reflect.Float64:
		return &openapi3.Schema{Type: &openapi3.Types{"number"}}
	case reflect.Bool:
		return &openapi3.Schema{Type: &openapi3.Types{"boolean"}}
	case reflect.Slice, reflect.Array:
		elem := t.Elem()
		items := app.fieldToSchema(reflect.StructField{Type: elem})
		return &openapi3.Schema{
			Type:  &openapi3.Types{"array"},
			Items: &openapi3.SchemaRef{Value: items},
		}
	case reflect.Struct:
		nested := app.generateSchema(reflect.New(t).Interface())
		nested.Nullable = nullable
		return nested
	case reflect.Map:
		return &openapi3.Schema{Type: schemaTypeObject}
	default:
		return &openapi3.Schema{Type: schemaTypeString}
	}
}
