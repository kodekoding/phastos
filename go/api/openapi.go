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

// segmentsAfterVersion strips the version prefix from a path and returns the remaining segments.
// E.g., "/v1/employee/absence/today" → ["employee", "absence", "today"]
func segmentsAfterVersion(path string) []string {
	path = strings.TrimPrefix(path, "/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		return nil
	}
	if !strings.HasPrefix(parts[0], "v") {
		return nil
	}
	segs := strings.Split(parts[1], "/")
	result := make([]string, 0, len(segs))
	for _, s := range segs {
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

// pathSegmentToTitle converts a path segment to a display-friendly title.
// E.g., "employee" → "Employee", "approval-line" → "Approval Line"
func pathSegmentToTitle(s string) string {
	s = strings.ReplaceAll(s, "-", " ")
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// isPathParam reports whether a path segment is a chi-style path parameter (e.g., "{id}").
func isPathParam(s string) bool {
	return len(s) > 2 && s[0] == '{' && s[len(s)-1] == '}'
}

// autoTag derives a group tag from a path by finding the deepest prefix
// (from 2 segments up to full length) that appears ≥ 2 times across all routes.
// Path parameters are excluded — a prefix ending with a param is not considered.
// E.g., for "/v1/employee/schedule/import/validate":
//
//	"employee"          → always base
//	"employee/schedule" → count 9 ≥ 3  → bestIdx = 2 ("Employee Schedule")
//	"employee/schedule/import" → count 3 ≥ 3 → bestIdx = 3 ("Employee Schedule Import")
//	"employee/schedule/import/validate" → count 1 < 3 → stop
//
// Result: "Employee Schedule Import"
func autoTag(path string, prefixCounts map[string]int) string {
	seg := segmentsAfterVersion(path)
	if len(seg) == 0 {
		return ""
	}
	bestIdx := 1
	for i := 2; i <= len(seg); i++ {
		if isPathParam(seg[i-1]) {
			break
		}
		prefix := strings.Join(seg[:i], "/")
		if prefixCounts[prefix] >= 2 {
			bestIdx = i
		}
	}
	parts := make([]string, bestIdx)
	for i := 0; i < bestIdx; i++ {
		parts[i] = pathSegmentToTitle(seg[i])
	}
	return strings.Join(parts, " ")
}

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
			Schemas: openapi3.Schemas{
				"ErrorResponse": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type: schemaTypeObject,
						Properties: openapi3.Schemas{
							"message": &openapi3.SchemaRef{Value: &openapi3.Schema{Type: schemaTypeString}},
							"code":    &openapi3.SchemaRef{Value: &openapi3.Schema{Type: schemaTypeString}},
							"data":    &openapi3.SchemaRef{Value: &openapi3.Schema{Nullable: true}},
						},
					},
				},
				"ResponseMetaData": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type: schemaTypeObject,
						Properties: openapi3.Schemas{
							"total_data":     &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &openapi3.Types{"integer"}}},
							"total_filtered": &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &openapi3.Types{"integer"}}},
						},
					},
				},
				"MessageResponse": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type: schemaTypeObject,
						Properties: openapi3.Schemas{
							"message": &openapi3.SchemaRef{Value: &openapi3.Schema{Type: schemaTypeString}},
						},
					},
				},
			},
		},
	}

	// Pre-register all named schemas from request/response types
	schemaNames := make(map[string]string)
	for _, entry := range app.routeRegistry {
		app.registerSchema(spec, entry.Doc.RequestType, schemaNames)
		app.registerSchema(spec, entry.Doc.ResponseType, schemaNames)
		app.registerSchema(spec, entry.Doc.SelectResponseType, schemaNames)
	}

	prefixCounts := make(map[string]int)
	for _, entry := range app.routeRegistry {
		seg := segmentsAfterVersion(entry.Path)
		for i := 2; i <= len(seg); i++ {
			if isPathParam(seg[i-1]) {
				break
			}
			prefix := strings.Join(seg[:i], "/")
			prefixCounts[prefix]++
		}
	}

	for _, entry := range app.routeRegistry {
		pathItem := spec.Paths.Find(entry.Path)
		if pathItem == nil {
			pathItem = &openapi3.PathItem{}
			spec.Paths.Set(entry.Path, pathItem)
		}
		app.buildOperation(pathItem, entry, prefixCounts, schemaNames)
	}

	for _, info := range app.middlewareDocs {
		if info.SecurityScheme != nil {
			if spec.Components.SecuritySchemes == nil {
				spec.Components.SecuritySchemes = openapi3.SecuritySchemes{}
			}
			scheme := &openapi3.SecurityScheme{}
			switch info.SecurityScheme.Type {
			case "bearer":
				scheme.Type = "http"
				scheme.Scheme = "bearer"
				scheme.BearerFormat = "JWT"
			case "apiKey":
				scheme.Type = "apiKey"
				scheme.Name = info.SecurityScheme.Name
				scheme.In = info.SecurityScheme.In
			default:
				scheme.Type = info.SecurityScheme.Type
				scheme.Name = info.SecurityScheme.Name
				scheme.In = info.SecurityScheme.In
			}
			spec.Components.SecuritySchemes[info.SecurityScheme.Name] = &openapi3.SecuritySchemeRef{
				Value: scheme,
			}
		}
	}

	return spec
}

func (app *App) buildOperation(item *openapi3.PathItem, entry routeRegistryEntry, prefixCounts map[string]int, schemaNames map[string]string) {
	operation := &openapi3.Operation{
		Summary:     entry.Doc.Summary,
		Description: entry.Doc.Description,
		Tags:        entry.Doc.Tags,
	}

	if len(operation.Tags) == 0 {
		if tag := autoTag(entry.Path, prefixCounts); tag != "" {
			operation.Tags = []string{tag}
		}
	}

	if entry.Doc.RequestType != nil {
		schemaRef := app.schemaRefOrValue(entry.Doc.RequestType, schemaNames)
		operation.RequestBody = &openapi3.RequestBodyRef{
			Value: openapi3.NewRequestBody().
				WithJSONSchemaRef(schemaRef).
				WithRequired(true),
		}
	}

	if entry.Doc.SelectResponseType != nil {
		itemRef := app.schemaRefOrValue(entry.Doc.SelectResponseType, schemaNames)
		schemaRef := generateSelectResponseSchema(itemRef)
		operation.AddResponse(200, openapi3.NewResponse().
			WithDescription("Success").
			WithJSONSchemaRef(&openapi3.SchemaRef{Value: schemaRef}),
		)
	} else if entry.Doc.ResponseType != nil {
		schemaRef := app.schemaRefOrValue(entry.Doc.ResponseType, schemaNames)
		operation.AddResponse(200, openapi3.NewResponse().
			WithDescription("Success").
			WithJSONSchemaRef(schemaRef),
		)
	}

	// Error responses (explicit)
	for _, errResp := range entry.Doc.ErrorResponses {
		operation.AddResponse(errResp.StatusCode, openapi3.NewResponse().
			WithDescription(errResp.Description).
			WithJSONSchemaRef(&openapi3.SchemaRef{
				Ref: "#/components/schemas/ErrorResponse",
			}),
		)
	}

	// Auto-generated error responses based on route configuration
	explicitCodes := make(map[int]bool)
	for _, er := range entry.Doc.ErrorResponses {
		explicitCodes[er.StatusCode] = true
	}
	for _, ar := range autoErrorResponses(entry) {
		if !explicitCodes[ar.StatusCode] {
			operation.AddResponse(ar.StatusCode, openapi3.NewResponse().
				WithDescription(ar.Description).
				WithJSONSchemaRef(&openapi3.SchemaRef{
					Ref: "#/components/schemas/ErrorResponse",
				}),
			)
		}
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

		if !field.IsExported() {
			continue
		}

		// Flatten embedded (anonymous) structs
		if field.Anonymous {
			ft := field.Type
			if ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				embedded := app.generateSchema(reflect.New(ft).Interface())
				for propName, propRef := range embedded.Properties {
					schema.Properties[propName] = propRef
				}
				schema.Required = append(schema.Required, embedded.Required...)
			}
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
		// Handle null.* wrapper types (null.String, null.Int64, etc.)
		if nullType := mapNullType(t); nullType != "" {
			return &openapi3.Schema{
				Type:     &openapi3.Types{nullType},
				Nullable: true,
			}
		}
		nested := app.generateSchema(reflect.New(t).Interface())
		nested.Nullable = nullable
		return nested
	case reflect.Map:
		return &openapi3.Schema{Type: schemaTypeObject}
	default:
		return &openapi3.Schema{Type: schemaTypeString}
	}
}

// mapNullType detects null.* wrapper types (null.String, null.Int64, etc.)
// and returns the underlying OpenAPI type name. Returns empty string for non-null types.
func mapNullType(t reflect.Type) string {
	if t.PkgPath() != "github.com/volatiletech/null" {
		return ""
	}
	switch t.Name() {
	case "String", "Byte":
		return "string"
	case "Int", "Int8", "Int16", "Int32", "Int64", "Uint", "Uint8", "Uint16", "Uint32", "Uint64":
		return "integer"
	case "Float32", "Float64":
		return "number"
	case "Bool":
		return "boolean"
	}
	return ""
}

// autoErrorResponses generates standard error responses based on route configuration.
func autoErrorResponses(entry routeRegistryEntry) []ErrorResponseDoc {
	var resp []ErrorResponseDoc

	hasBinding := entry.Doc.RequestType != nil || len(entry.PathParamTypes) > 0
	if hasBinding {
		resp = append(resp, ErrorResponseDoc{
			StatusCode:  400,
			Code:        "ERROR_VALIDATION",
			Description: "Binding/validation failed",
		})
	}

	if entry.Doc.Security != nil {
		resp = append(resp, ErrorResponseDoc{
			StatusCode:  401,
			Code:        "UNAUTHORIZED",
			Description: "Missing or invalid authorization",
		})
		resp = append(resp, ErrorResponseDoc{
			StatusCode:  403,
			Code:        "FORBIDDEN",
			Description: "Forbidden access",
		})
	}

	resp = append(resp,
		ErrorResponseDoc{
			StatusCode:  422,
			Code:        "UNPROCESSABLE_ENTITY",
			Description: "Business logic / processing error",
		},
		ErrorResponseDoc{
			StatusCode:  500,
			Code:        "INTERNAL_SERVER_ERROR",
			Description: "Unhandled server error",
		},
	)

	return resp
}

// registerSchema adds a named schema to the OpenAPI Components.Schemas
// if not already present. Returns the schema name.
func (app *App) registerSchema(spec *openapi3.T, model any, names map[string]string) string {
	if model == nil {
		return ""
	}
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() == reflect.Slice {
		return app.registerSchema(spec, reflect.New(t.Elem()).Elem().Interface(), names)
	}
	if t.Kind() != reflect.Struct {
		return ""
	}
	key := t.PkgPath() + "." + t.Name()
	if name, ok := names[key]; ok {
		return name
	}
	schemaName := t.Name()
	names[key] = schemaName
	schema := app.generateSchema(model)
	spec.Components.Schemas[schemaName] = &openapi3.SchemaRef{Value: schema}
	return schemaName
}

// schemaRefOrValue returns a $ref to a named schema if available,
// otherwise returns the inline schema value.
func (app *App) schemaRefOrValue(model any, names map[string]string) *openapi3.SchemaRef {
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() == reflect.Slice {
		elem := t.Elem()
		if elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		if elem.Kind() == reflect.Struct {
			key := elem.PkgPath() + "." + elem.Name()
			if name, ok := names[key]; ok {
				return &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:  &openapi3.Types{"array"},
						Items: &openapi3.SchemaRef{Ref: "#/components/schemas/" + name},
					},
				}
			}
		}
	}
	if t.Kind() == reflect.Struct {
		key := t.PkgPath() + "." + t.Name()
		if name, ok := names[key]; ok {
			return &openapi3.SchemaRef{Ref: "#/components/schemas/" + name}
		}
	}
	return &openapi3.SchemaRef{Value: app.generateSchema(model)}
}

// generateSelectResponseSchema creates a schema for list endpoints.
// It flattens the embedded ResponseMetaData (total_data, total_filtered, request_param)
// together with the typed data field.
func generateSelectResponseSchema(itemRef *openapi3.SchemaRef) *openapi3.Schema {
	schema := &openapi3.Schema{
		Type:       schemaTypeObject,
		Properties: openapi3.Schemas{},
	}

	// data field with the caller's item type
	schema.Properties["data"] = itemRef

	// Flatten ResponseMetaData fields (total_data, total_filtered, request_param)
	schema.Properties["total_data"] = &openapi3.SchemaRef{
		Value: &openapi3.Schema{Type: &openapi3.Types{"integer"}},
	}
	schema.Properties["total_filtered"] = &openapi3.SchemaRef{
		Value: &openapi3.Schema{Type: &openapi3.Types{"integer"}},
	}
	schema.Properties["request_param"] = &openapi3.SchemaRef{
		Value: &openapi3.Schema{Type: schemaTypeObject},
	}

	return schema
}
