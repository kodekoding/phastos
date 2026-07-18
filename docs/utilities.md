# Utilities

Phastos utilities provide file generators, cloud storage (GCS), CSV/Excel data importers, authentication middleware, environment detection, structured logging, and a collection of helpers for JWT, encryption, Slack notifications, templating, and more.

## File Generators

`go/generator/` — Unified file generation via the `FileGenerator` interface (`go/generator/generator.go:20`).

```go
import "github.com/kodekoding/phastos/v2/go/generator"
```

### FileGenerator Interface

```go
type FileGenerator interface {
    Generate() error
    FileName() string
}
```

All generator types (PDF, CSV, Excel, QR, Banner) implement this interface, enabling polymorphic file generation:

```go
var gen generator.FileGenerator
switch outputType {
case "pdf":
    gen = pdfInstance
case "csv":
    gen = csvInstance
case "excel":
    gen = excelInstance
}
if err := gen.Generate(); err != nil { /* ... */ }
fmt.Println("output:", gen.FileName())
```

### PDF Generator

Requires `wkhtmltopdf` binary installed on the system. Uses `go-wkhtmltopdf` to convert HTML templates into PDF documents.

```go
pdf, err := generator.NewPDF(&generator.ConverterOptions{
    PageSize:     "A4",
    MarginBottom: 10,
    MarginTop:    10,
    MarginLeft:   11,
    MarginRight:  11,
})
if err != nil {
    log.Fatal(err)
}

pdf.
    AddCustomFunction("formatDate", myFormatFunc).
    SetTemplate("templates/report.html", templateData).
    SetFooterHTMLTemplate("templates/footer.html").
    SetFileName(&outputPath).
    Generate()
```

| Interface `PDFs` | Description |
|---|--|
| `NewPDF(opts)` | Creates PDF generator. Optional `*ConverterOptions` for page size and margins |
| `SetTemplate(path, data)` | Parses an HTML template with `data` and renders it to string content |
| `SetFooterHTMLTemplate(path)` | Sets an HTML footer with configurable spacing |
| `SetFileName(*string)` | Sets the output filename (auto-creates `tmp/pdf/` directory) |
| `AddCustomFunction(name, func)` | Registers a custom template function for use in HTML templates |
| `Generate()` | Renders HTML to PDF and writes to `fileName` |
| `FileName()` | Returns the output file path |
| `Error()` | Returns accumulated errors from chained calls |

### CSV Generator

```go
csv := generator.NewCSV()

csv.
    SetHeader([]string{"Name", "Email", "Role"}).
    AppendDataRow([]string{"Alice", "alice@example.com", "Admin"}).
    AppendDataRow([]string{"Bob", "bob@example.com", "User"}).
    Generate()

path := csv.FileName() // "generated-csv.csv"
```

| Interface `CSVs` | Description |
|---|--|
| `NewCSV()` | Creates CSV generator with default filename `generated-csv.csv` |
| `SetFileName(name)` | Sets the output filename (`.csv` suffix is appended automatically) |
| `SetHeader([]string)` | Prepends a header row; validates column count matches existing rows |
| `AppendDataRow([]string)` | Appends a data row; validates column count matches header |
| `Generate()` | Writes content to the CSV file |
| `FileName()` | Returns the output file path |
| `Error()` | Returns accumulated validation errors |

### Excel Generator

Operates in three modes: new file, open from path, and open from upload. Uses `excelize` for .xlsx files.

```go
// Create a new Excel file
xls := generator.NewExcel(&generator.ExcelOptions{Source: "new"})

xls.
    SetSheetName("Users").
    SetHeader([]string{"Name", "Email", "Role"}).
    AppendDataRow([]string{"Alice", "alice@example.com", "Admin"}).
    AppendDataRow([]string{"Bob", "bob@example.com", "User"}).
    SetFileName("users-report").
    Generate()

// Open an existing file
xls := generator.NewExcel(&generator.ExcelOptions{
    Source: "path",
    File:   "/path/to/file.xlsx",
})

// Open from HTTP upload
xls := generator.NewExcel(&generator.ExcelOptions{
    Source: "upload",
    File:   uploadedMultipartFile,
})
```

| Interface `Excels` | Description |
|---|--|
| `NewExcel(opts)` | Creates Excel handler. `Source`: `"new"`, `"path"`, or `"upload"` |
| `SetFileName(name)` | Sets output filename (`.xlsx` suffix appended) |
| `SetSheetName(name)` | Sets the active sheet name (default `"Sheet1"`) |
| `SetHeader([]string)` | Prepends a header row with bold + center styling |
| `AppendDataRow([]string)` | Appends a data row |
| `Generate()` | Creates sheet, writes content, applies header styling, saves as `.xlsx` |
| `WriteToBuffer()` | Writes Excel to `*bytes.Buffer` instead of file (for HTTP response streaming) |
| `GetContents(sheetName)` | Reads all rows as `[]map[string]string` with header keys |
| `ScanContentToStruct(sheetName, &dest)` | Reads sheet and JSON-unmarshals into a pointer struct |
| `GetMergeCell(sheetName)` | Returns merged cells in the sheet |
| `GetExcelFile()` | Returns underlying `*excelize.File` for advanced operations |
| `FileName()` | Returns output file path |
| `Error()` | Returns accumulated errors |

### QR Code Generator

Uses `yeqown/go-qrcode` to generate QR code images. Supports embedding a logo in the center.

```go
qr, err := generator.NewQR("https://example.com/verify/abc123")
if err != nil {
    log.Fatal(err)
}

qr.
    SetLogoImg("assets/logo.png").
    SetFileName(&outputPath).
    Generate()

path := qr.FileName() // tmp/qr/<md5-hash>.jpeg
```

| Interface `QRs` | Description |
|---|--|
| `NewQR(content)` | Creates a QR code for the given string content |
| `SetLogoImg(path)` | Embeds a logo image in the center of the QR code |
| `SetFileName(*string)` | Sets output filename (auto-creates `tmp/qr/` with MD5-hashed name) |
| `Generate()` | Saves QR code as JPEG to disk |
| `FileName()` | Returns the output file path |

### Banner Image Generator

Uses `fogleman/gg` for 2D graphics. Composes banners with background color, image layers, and text labels.

```go
banner := generator.NewBanner(
    generator.WithWidth(1200),
    generator.WithHeight(400),
    generator.WithBackgroudColor("#1a1a2e"),
)

banner.
    AddImageLayer(&generator.ImageLayer{
        Image: logo,
        XPos:  50,
        YPos:  50,
    }).
    AddLabel(&generator.Label{
        Text:     "Certificate of Completion",
        FontPath: "fonts/Roboto-Bold.ttf",
        Size:     48,
        Color:    color.White,
        XPos:     600,
        YPos:     200,
    }).
    Generate().
    Save("output/certificate.png")
```

| Interface `Banners` | Description |
|---|--|
| `NewBanner(opts...)` | Creates banner. Options: `WithWidth`, `WithHeight`, `WithBackgroudColor` (hex). Default: 1200×400, white |
| `AddImageLayer(img)` | Adds an image layer (higher index = upper layer) |
| `AddLabel(label)` | Adds a text label with font, size, color, and position |
| `Generate()` | Renders background + layers + labels into `image.Image` |
| `Save(destPath)` | Saves to PNG, JPEG, or GIF based on file extension |
| `SetDestPath(path)` | Sets destination path (alternative to passing path to `Save`) |
| `Image()` | Returns the rendered `image.Image` |
| `FileName()` | Returns the destination path |

---

## Cloud Storage (GCS)

`go/storage/` — Google Cloud Storage client implementing the `Buckets` interface.

```go
import "github.com/kodekoding/phastos/v2/go/storage"
```

### Buckets Interface (`go/storage/definition.go:10`)

```go
type Buckets interface {
    UploadImage(ctx context.Context, file multipart.File, fileName *string) error
    UploadFile(ctx context.Context, file multipart.File, fileName *string) error
    UploadImageFromLocalPath(ctx context.Context, filePath string, fileName *string, deleteAfterSuccess ...bool) error
    UploadFileFromLocalPath(ctx context.Context, filePath string, fileName *string, deleteAfterSuccess ...bool) error
    UploadImagePublic(ctx context.Context, file multipart.File, fileName *string) error
    UploadFilePublic(ctx context.Context, file multipart.File, fileName *string) error
    UploadImageFromLocalPathPublic(ctx context.Context, filePath string, fileName *string, deleteAfterSuccess ...bool) error
    UploadFileFromLocalPathPublic(ctx context.Context, filePath string, fileName *string, deleteAfterSuccess ...bool) error
    GetSignedURLFile(ctx context.Context, imgPath string) (signedUrl string, err error)
    GetFileFS(ctx context.Context, filePath string) (fs.File, error)
    SetFileExpiredTime(minutes int) Buckets
    SetBucketName(fileName string) Buckets
    SetContentType(contentType string) Buckets
    RollbackProcess(ctx context.Context, fileName string) error
    DeleteFile(ctx context.Context, fileName string) error
    CopyFileToAnotherBucket(ctx context.Context, destBucket, fileName string) error
    GenerateSignedURL(urlType string, path string, expires ...time.Duration) (string, error)
    Close()
}
```

### Initialization

Requires `STORAGE_CREDENTIALS_PATH` (or `GOOGLE_APPLICATION_CREDENTIALS`) environment variable pointing to a GCP service account JSON key file.

```go
ctx := context.Background()
gcs, err := storage.NewGCS(ctx, "my-bucket-name")
if err != nil {
    log.Fatal(err)
}
defer gcs.Close()
```

### Upload Methods

All upload methods prepend the path with `{private|public}/{img|file}/{env}` (e.g., `private/img/local/photo.jpg`). The `*fileName` pointer is updated in-place with the full GCS object path.

| Method | Source | Visibility |
|---|---|---|
| `UploadImage(ctx, file, fileName)` | `multipart.File` | private |
| `UploadFile(ctx, file, fileName)` | `multipart.File` | private |
| `UploadImageFromLocalPath(ctx, path, fileName, delete?)` | local file path | private |
| `UploadFileFromLocalPath(ctx, path, fileName, delete?)` | local file path | private |
| `UploadImagePublic(ctx, file, fileName)` | `multipart.File` | public (allUsers reader ACL) |
| `UploadFilePublic(ctx, file, fileName)` | `multipart.File` | public |
| `UploadImageFromLocalPathPublic(ctx, path, fileName, delete?)` | local file path | public |
| `UploadFileFromLocalPathPublic(ctx, path, fileName, delete?)` | local file path | public |

`deleteAfterSuccess` (variadic `...bool`): when `true`, the local file is removed after successful upload. Default: `false`.

### Signed URLs

```go
// Simple signed URL for file download (default 60-min expiry)
url, err := gcs.GetSignedURLFile(ctx, "private/img/local/photo.jpg")

// Generate signed URL for upload with custom expiry
uploadURL, err := gcs.GenerateSignedURL(storage.UploadProcess, "uploads/file.pdf", 10*time.Minute)

// Generate signed URL for download
downloadURL, err := gcs.GenerateSignedURL(storage.DownloadProcess, "reports/report.xlsx", 30*time.Minute)
```

| Method | Description |
|---|---|
| `GetSignedURLFile(ctx, path)` | Generates a GET signed URL. Expiry controlled by `SetFileExpiredTime` (default 60 min) |
| `GenerateSignedURL(urlType, path, expires...)` | Generates signed URL. `urlType`: `storage.UploadProcess` (PUT) or `storage.DownloadProcess` (GET). Upload default expiry: 1 min. Download default: 5 min |
| `SetFileExpiredTime(minutes)` | Sets expiry in minutes for `GetSignedURLFile` |

### File Management

```go
// Copy file to another bucket
err := gcs.CopyFileToAnotherBucket(ctx, "archive-bucket", "private/img/local/doc.pdf")

// Delete a file
err := gcs.DeleteFile(ctx, "private/img/local/doc.pdf")

// Rollback (alias for DeleteFile)
err := gcs.RollbackProcess(ctx, "private/img/local/doc.pdf")

// Read file as io/fs.File (supports io/fs interfaces)
f, err := gcs.GetFileFS(ctx, "templates/email.html")

// Set content type for subsequent uploads
gcs.SetContentType("application/pdf")

// Switch to a different bucket
gcs.SetBucketName("another-bucket")
```

### Google Drive

```go
drive, err := storage.NewDrive(ctx)
```

Uses `DRIVE_CREDENTIALS_PATH` (or `GOOGLE_APPLICATION_CREDENTIALS`) to initialise a `*drive.Service` for Google Drive API operations.

---

## Data Importer

`go/importer/` — CSV/Excel data import with configurable worker pool, transaction-per-batch processing, struct validation, progress tracking, and optional Slack notifications.

```go
import "github.com/kodekoding/phastos/v2/go/importer"
```

### Quick Start

```go
type UserImport struct {
    Name  string `csv:"Name"`
    Email string `csv:"Email"`
    Role  string `csv:"Role"`
}

result := importer.New(
    importer.WithCtx(ctx),
    importer.WithFile(uploadedFile),
    importer.WithExtFile(".xlsx"),
    importer.WithStructDestination(UserImport{}),
    importer.WithTransaction(db),
    importer.WithWorker(10),
    importer.WithProcessName("user-import"),
    importer.WithProcessFn(func(ctx context.Context, singleData interface{}, trx *sqlx.Tx, workerIdx int) *api.HttpError {
        user := singleData.(*UserImport)
        _, err := db.Write(ctx, &database.QueryOpts{
            CUDRequest: constructInsert(user),
            Trx:        trx,
        })
        if err != nil {
            return api.NewErr(api.WithErrorData(user), api.WithErrorStatus(500), api.WithErrorMessage(err.Error()))
        }
        return nil
    }),
    importer.WithSentNotifToSlack(true),
).ProcessData()

fmt.Printf("Total: %d, Success: %d, Failed: %d, Time: %.2fs\n",
    result.TotalData, result.TotalSuccess, result.TotalFailed, result.ExecutionTime)
```

### ImportOptions

| Option | Description |
|---|---|
| `WithCtx(ctx)` | Parent context (required) |
| `WithFile(file)` | `multipart.File` from HTTP upload (required) |
| `WithExtFile(ext)` | File extension: `".xls"`, `".xlsx"`, or `".csv"` (required) |
| `WithStructDestination(struct)` | Target struct for field mapping via `csv`/`excel` tags (required for regular import) |
| `WithTransaction(trx)` | Database transaction provider (`database.Transactions`) (required) |
| `WithWorker(n)` | Number of concurrent worker goroutines (default: 10) |
| `WithProcessName(name)` | Name for logging and Slack notification title |
| `WithProcessFn(fn)` | Processing function `func(ctx, data, trx, workerIdx) *api.HttpError` (required) |
| `WithSentNotifToSlack(bool, channel?)` | Send Slack notification on completion. Optional custom webhook channel |
| `WithSheetName(name)` | Sheet name for Excel files (default: first sheet) |

### ImportResult

```go
type ImportResult struct {
    FailedList       map[string][]interface{} // grouped by error message
    SuccessList      []interface{}
    TotalData        int
    TotalFailed      int
    TotalSuccess     int
    ExecutionTime    float64                  // seconds
    UniqueProcessKey string
}
```

### Processing Pipeline

Each row flows through this pipeline:

1. **File Reader** — reads headers + rows from CSV or Excel, parses each row into the destination struct via JSON marshal/unmarshal, sends through `chan rowData`
2. **Worker Pool** — `n` goroutines consume from the channel. Each worker:
   - Validates the struct via `api.ValidateStruct()`
   - Begins a transaction via `trx.Begin()`
   - Calls the user's `processFn`
   - Calls `trx.Finish(trx, err)` — commits on nil error, rolls back otherwise
   - Sends `processedResult` to the result channel
3. **Aggregator** — collects results, builds `FailedList` and `SuccessList`, sends Slack notification if enabled
4. **Slack Notification** — sends summary with total data, failed count, execution time, and JWT user info

The pipeline uses `sync.Pool` for `processedResult` objects to minimise GC pressure.

### Pivot Data Import

For files with cross-tab/pivot layouts where column headers represent data values:

```go
result := importer.New(
    importer.WithCtx(ctx),
    importer.WithFile(uploadedFile),
    importer.WithExtFile(".xlsx"),
    importer.WithSheetName("Sheet1"),
    // Pivot configuration
    importer.WithHeaderRowIndex(0),    // row index of column headers
    importer.WithDataStartRow(1),       // first data row (after header)
    importer.WithKeyColumns([]int{0, 1}), // columns that form the composite key
    importer.WithKeySeparator("|"),
    importer.WithValueStartCol(2),      // first column containing values
    importer.WithOnPivotEntry(func(key, value string) {
        fmt.Printf("Processing: %s = %s\n", key, value)
    }),
    importer.WithTransaction(db),
    importer.WithWorker(10),
    importer.WithProcessName("pivot-import"),
    importer.WithProcessFn(func(ctx context.Context, singleData interface{}, trx *sqlx.Tx, workerIdx int) *api.HttpError {
        entry := singleData.(map[string]any)
        // entry["Employee ID"], entry["Department"], entry["pivot_header"], entry["pivot_value"]
        return nil
    }),
    importer.WithSentNotifToSlack(true),
).ProcessPivotData()
```

Each entry in `processFn` is a `map[string]any` containing:
- All key column values (e.g., `"Employee ID": "444201123"`)
- `"pivot_header"` — the value column header (e.g., `"2024-03-26"`)
- `"pivot_value"` — the cell value (e.g., `"HONS"`)

Use `ReadPivotData()` instead of `ProcessPivotData()` to get a flat `map[string]string` result without worker-pool processing.

### Helper Functions

```go
// Get all rows from .xlsx file
rows, err := importer.GetDataFromXlsx(uploadedFile, "Sheet1")

// Get all rows from .xls file
rows, err := importer.GetDataFromXls(uploadedFile)
```

---

## Auth Middleware

`go/middlewares/` — HTTP middleware for JWT authentication, static secret-based auth, and rate limiting.

```go
import "github.com/kodekoding/phastos/v2/go/middlewares"
```

### JWTAuth

Validates `Authorization: Bearer <token>` header using HS256 signing. Requires `JWT_SIGNING_KEY` environment variable. On success, stores claims in request context via `context.SetJWT(r, claims)`.

```go
// Register as route middleware
r := api.NewRoute("GET", myHandler, api.WithMiddleware(middlewares.JWTAuth))

// Access claims in handler
claims := context.GetJWT(r)
fmt.Println(claims.Data)   // custom data embedded in JWT
fmt.Println(claims.Token)  // raw token string
```

| Detail | Value |
|---|---|
| Auth header | `Authorization: Bearer <token>` |
| Signing algorithm | HS256 |
| Env variable | `JWT_SIGNING_KEY` |
| Claims struct | `entity.JWTClaimData` (stored in context) |
| Context access | `context.GetJWT(r)` |
| Error codes | `INVALID_KEY`, `INVALID_CLAIMS`, `TOKEN_NOT_VALID`, `INVALID_STRUCT_CLAIM` |

### StaticAuth

Validates a secret header against `SERVICE_SECRET` environment variable. Useful for service-to-service communication.

```go
r := api.NewRoute("POST", internalHandler, api.WithMiddleware(middlewares.StaticAuth))
```

| Detail | Value |
|---|---|
| Auth header | `Secret: <token>` (constant `common.HeaderSecret`) |
| Env variable | `SERVICE_SECRET` |
| Error message | "Invalid Token" / `ERR_INVALID_TOKEN` |

### RateLimiter

Token-bucket rate limiter per IP address using `golang.org/x/time/rate`. Returns 429 Too Many Requests when limit exceeded.

```go
// Default: 10 rps, burst 20
limiter := middlewares.NewRateLimiter()

// Custom rate
limiter := middlewares.NewRateLimiter(
    middlewares.WithRate(100, 200),        // 100 rps, burst 200
    middlewares.WithSkipPaths("/ping", "/health"),
    middlewares.WithMessage("Too many requests, calm down", "CUSTOM_RATE_LIMIT"),
)

// Custom key extractor (e.g., by API key instead of IP)
limiter := middlewares.NewRateLimiter(
    middlewares.WithKeyExtractor(func(r *http.Request) string {
        return r.Header.Get("X-API-Key")
    }),
)

r := api.NewRoute("GET", handler, api.WithMiddleware(limiter))
```

| Option | Description |
|---|---|
| `NewRateLimiter(opts...)` | Creates rate limiter middleware. Default: 10 rps, burst 20, per-IP |
| `WithRate(rps, burst)` | Sets requests-per-second limit and burst size |
| `WithSkipPaths(paths...)` | Disables rate limiting for specific exact paths |
| `WithMessage(msg, code)` | Custom error message and error code (default: `"rate limit exceeded"`, `"RATE_LIMITED"`) |
| `WithKeyExtractor(fn)` | Custom function to extract the rate-limit bucket key (default: client IP from X-Forwarded-For, X-Real-Ip, or RemoteAddr) |

---

## Environment

`go/env/` — Environment detection utilities. Reads from `APPS_ENV` environment variable and `.env` file on startup.

```go
import "github.com/kodekoding/phastos/v2/go/env"
```

### Service Environment

```go
env := env.ServiceEnv() // "local" | "development" | "staging" | "production"

if env.IsProduction() { /* ... */ }
if env.IsStaging() { /* ... */ }
if env.IsDevelopment() { /* ... */ }
if env.IsLocal() { /* ... */ }
```

| Constant | Value |
|---|---|
| `LocalEnv` | `"local"` |
| `DevelopmentEnv` | `"development"` |
| `StagingEnv` | `"staging"` |
| `ProductionEnv` | `"production"` |

| Function | Returns |
|---|---|
| `ServiceEnv()` | Current env string; defaults to `"development"` if unset |
| `IsLocal()` | `true` when `APPS_ENV=local` |
| `IsDevelopment()` | `true` when `APPS_ENV=development` |
| `IsStaging()` | `true` when `APPS_ENV=staging` |
| `IsProduction()` | `true` when `APPS_ENV=production` |
| `GoVersion()` | Go runtime version string |
| `SetFromEnvFile(filepath)` | Reads a `.env` file and sets OS environment variables (called automatically on `init()`) |

---

## Logging

`go/log/` — Structured logging via `rs/zerolog` with New Relic and OpenTelemetry integration.

```go
import "github.com/kodekoding/phastos/v2/go/log"
```

### Initialization

Call `log.Get()` early in `main()`. Uses `sync.Once` — subsequent calls return the same logger instance.

```go
// Basic: console output (colored, development-friendly)
logger := log.Get(log.WithAppPort(8080))

// With New Relic log correlation
logger := log.Get(
    log.WithAppPort(8080),
    log.WithAppVersion("v1.2.3"),
    log.WithNewRelicApp(nrApp),
)

// With OpenTelemetry log forwarding
logger := log.Get(
    log.WithAppPort(8080),
    log.WithOTelLogEndpoint(),
)
```

| Option | Description |
|---|---|
| `WithAppPort(port)` | Application port, included in every log entry |
| `WithAppVersion(version)` | Application version, included in every log entry |
| `WithNewRelicApp(app)` | Injects New Relic trace/span IDs into log entries |
| `WithOTelLogEndpoint()` | Forwards logs to OpenTelemetry collector via TCP (port 54526, derived from `OTEL_EXPORTER_OTLP_ENDPOINT`) |

### Writers by Environment

| Environment | Writer Configuration |
|---|---|
| **Production** | JSON writer (stdout) + optional NR log correlation + optional OTel TCP writer. Log level: `InfoLevel` |
| **Non-production** | Colored console writer (stdout) + optional NR/OTel. Log level: `DebugLevel` |

### Context Logger

```go
func handler(w http.ResponseWriter, r *http.Request) {
    logger := log.Ctx(r.Context())
    logger.Info().
        Str("user_id", "123").
        Int("items", 5).
        Msg("Order created")
}
```

`log.Ctx(ctx)` returns `*zerolog.Logger` from context. The request logger middleware (registered automatically by `api.App.Init()`) stores the logger with trace context in each request's context.

Automatic fields on every log entry: `app`, `env`, `port`, `app_version` (if set), `container_name` (if `CONTAINER_NAME` env is set).

---

## Helpers

`go/helper/` — General-purpose utilities for JWT, encryption, Slack notifications, templating, UUID generation, and more.

```go
import "github.com/kodekoding/phastos/v2/go/helper"
```

### JWT

```go
// Generate JWT with custom claims data
token, err := helper.GenerateJWTToken(
    map[string]interface{}{"user_id": 42, "role": "admin"},
    2*time.Hour, // optional: custom expiry (default: 24h)
)
```

| Function | Description |
|---|---|
| `GenerateJWTToken(data, expireTime...)` | Creates HS256 JWT with `entity.JWTClaimData` claims. Uses `JWT_SIGNING_KEY` env and `JWT_ISSUER` env (default: `"phastos"`) |

### Encryption (AES-256-GCM)

```go
// From an explicit key
cm, err := helper.NewCryptoManager("my-32-byte-encryption-secret-key")
if err != nil {
    log.Fatal(err)
}

// From an environment variable
cm, err := helper.NewCryptoManagerFromEnv("ENCRYPTION_KEY")

// Encrypt
ciphertext, err := cm.Encrypt("sensitive-api-key-value")

// Decrypt
plaintext, err := cm.Decrypt(ciphertext)

// Generate a public key hash (safe for client-side storage)
publicKey := helper.GeneratePublicKey(plaintext)

// Verify a key against its public hash
valid := helper.VerifyAPIKey(plaintext, publicKey)
```

| Type/Method | Description |
|---|---|
| `NewCryptoManager(key)` | Creates AES-256-GCM crypto manager. Key is hashed with SHA-256 to ensure 32 bytes |
| `NewCryptoManagerFromEnv(envVar)` | Creates from environment variable |
| `Encrypt(plaintext)` | Encrypts with random nonce, returns base64-encoded ciphertext |
| `Decrypt(ciphertext)` | Decrypts base64-encoded ciphertext |
| `GeneratePublicKey(apiKey)` | SHA-256 hash of the plaintext key, base64-encoded |
| `VerifyAPIKey(apiKey, publicKey)` | Verifies key against stored public key hash |

### Slack Notifications

```go
ctx := r.Context()

err := helper.SendSlackNotification(ctx,
    helper.NotifTitle("Import Complete"),
    helper.NotifMsgType(helper.NotifInfoType),
    helper.NotifData(map[string]string{
        "Total Data":    "1000",
        "Success":       "995",
        "Failed":        "5",
        "Execution Time": "3.42 second(s)",
    }),
    helper.NotifChannel("https://hooks.slack.com/services/..."),
)
```

| Type | Description |
|---|---|
| `NotifInfoType` | Green color (`#2fe329`), info icon |
| `NotifWarnType` | Yellow color (`#f7bf31`), warning icon |
| `NotifErrorType` | Red color (`#ff0e0a`), broken-heart icon |
| `NotifTitle(title)` | Sets notification title |
| `NotifMsgType(type)` | Sets type: `info`, `warn`, `error` |
| `NotifData(map[string]string)` | Key-value data fields. Keys prefixed with `-` disable `Short` field (full-width) |
| `NotifChannel(url)` | Custom webhook URL (overrides context's default Slack destination) |

### UUID and Random String

```go
// UUID v7 (google/uuid)
id := helper.GenerateUUID() // e.g., "0190c9a7-8f5e-7b3a-9c1d-2e4f6a8b0c3d"

// UUID v4 (satori/go.uuid)
id := helper.GenerateUUIDV4() // e.g., "a1b2c3d4-e5f6-7890-abcd-ef1234567890"

// Random alphanumeric string
str := helper.GenerateRandomString(16) // e.g., "kL9mN2xR7vB5qW8p"

// Random string with custom charset
str := helper.GenerateRandomStringWithCharset(8, "0123456789ABCDEF")

// Fast nanoid-style ID (crypto/rand, ~15 chars, pool-allocated)
id := helper.GenerateFastID() // e.g., "aB3xK_9mN2-rV8pL"

// Ultra-fast counter-based ID (not cryptographically secure)
id := helper.GenerateFastIDCounter()       // returns string
buf := helper.GenerateFastIDCounterBytes() // returns *[]byte (pool-allocated, zero heap alloc)
helper.PutFastIDCounterBytes(buf)          // return buffer to pool after use
```

| Function | Length | Source | Notes |
|---|---|---|---|
| `GenerateUUID()` | 36 chars (standard) | `google/uuid` v7 | Cryptographically random |
| `GenerateUUIDV4()` | 36 chars (standard) | `satori/go.uuid` v4 | Cryptographically random |
| `GenerateRandomString(n)` | `n` chars | `math/rand` | Alphanumeric (`A-Za-z0-9`) |
| `GenerateRandomStringWithCharset(n, charset)` | `n` chars | `math/rand` | Custom charset |
| `GenerateFastID()` | 15 chars | `crypto/rand` | Nanoid-style, 64-char alphabet, zero-bias mask |
| `GenerateFastIDCounter()` | 15 chars | atomic counter | Not crypto-secure, sequential, ~15ns |

### Template Parsing

```go
// Parse from embed.FS
var templates embed.FS
result, err := helper.ParseTemplate(templates, "slack/notif.json", data)
slackPayload := result.String()

// Parse from filesystem path
result, err := helper.ParseFileTemplate("/templates/email.html", data, "<html>", "</html>")

// Parse from path (supports local files AND HTTP URLs) with custom functions
result, err := helper.ParseTemplateFromPath(
    "https://cdn.example.com/templates/report.html",
    data,
    template.FuncMap{"formatDate": myFormatFunc},
    "<!-- additional body content -->",
)
```

| Function | Description |
|---|---|
| `ParseTemplate(embedFS, file, data, additionalBodyContent...)` | Parses template from `embed.FS` |
| `ParseFileTemplate(path, data, additionalBodyContent...)` | Parses from filesystem using `html/template` |
| `ParseTemplateFromPath(path, data, optionalParams...)` | Parses from local file or HTTP URL. Supports `template.FuncMap`, `map[string]any` func maps, and `string` body content |
| `GetTemplate(file, data, destStruct)` | Parses template from file path and JSON-unmarshals into a struct |
| `GetTemplateFS(embedFS, file, data, destStruct)` | Same as `GetTemplate` but from `embed.FS` |

### Struct Reflection Utilities

```go
// Generate INSERT columns/values from a struct
cols, values := helper.ConstructColNameAndValue(ctx, myStruct)

// Generate UPDATE SET clause from a struct (cached template for repeated use)
updateData := helper.ConstructColNameAndValueForUpdate(ctx, updateStruct)
// updateData.Cols = ["name=?", "email=?", "updated_at=?"]
// updateData.Values = ["Alice", "alice@example.com", "2024-01-15 10:30:00"]
// updateData.ColsInsert = "name,email,updated_at"

// Generate bulk INSERT from a slice of structs
bulkData, err := helper.ConstructColNameAndValueBulk(ctx, []User{user1, user2, user3})

// Generate SELECT column list from a struct (cached)
cols := helper.GenerateSelectCols(ctx, User{})
cols := helper.GenerateSelectCols(ctx, User{}, helper.WithExcludedCols("password,salt"))

// Convert struct to map[string]interface{} using csv/excel tags
m := helper.ConvertStructToMap(myStruct)
```

| Function | Description |
|---|---|
| `ConstructColNameAndValue(ctx, struct)` | Returns `(columns []string, values []interface{})` using `db` tag mapping. Handles null types, pointers, embedded structs, skip tags |
| `ConstructColNameAndValueForUpdate(ctx, struct, extraValues...)` | Returns `*CUDConstructData` with cached template-based column/value extraction. Auto-adds `updated_at=?` if not present |
| `ConstructColNameAndValueBulk(ctx, slice, conditions...)` | Returns `*CUDConstructData` for bulk INSERT or bulk UPDATE with JOIN conditions |
| `GenerateSelectCols(ctx, struct, opts...)` | Returns `[]string` of column names. Supports `WithExcludedCols`, `WithIncludedCols`, recursive embedded structs. Results are cached per struct type |
| `ConvertStructToMap(struct)` | Converts struct to `map[string]interface{}` using `csv`, `excel`, or field name tags |

### String Case Conversion

```go
helper.ToCamelCase("any_kind_of_string")     // "AnyKindOfString"
helper.ToLowerCamelCase("any_kind_of_string") // "anyKindOfString"
helper.ToSnakeCase("AnyKindOfString")         // "any_kind_of_string"
helper.TrimDuplicatedSpace("hello   world")   // "hello world"
```

### Slice Utilities

```go
helper.SliceContains([]string{"a", "b", "c"}, "b") // true
helper.Remove(slice, 2) // removes element at index 2
```

### Color Parsing

```go
rgba, err := helper.ParseHexColor("#ff5733")  // color.RGBA{R:255, G:87, B:51, A:255}
rgba, err := helper.ParseHexColor("#f53")     // short form: #ff5533
uniform := helper.GetColorUniform("#1a1a2e")  // *image.Uniform{R:26, G:26, B:46, A:255}
```

### File Path Utilities

```go
// Get base file path (returns "files" in local env, "" otherwise)
base := helper.GetFilePath()

// Ensure directory exists for a file path
helper.CheckFolder("/tmp/pdf/output/report.pdf")

// Split path into folder and filename
folder, filename := helper.GetFolderAndFileName("/tmp/pdf/report.pdf")
// folder = "tmp/pdf/", filename = "report.pdf"

// Get temp folder path (creates if missing)
tmpPath, err := helper.GetTmpFolderPath() // "files/tmp" or "/tmp"
```

### Upload to Temp

```go
// Upload to local temp folder
err := helper.UploadToTmp(ctx, &filePath, uploadedFile)

// Upload to GCS temp bucket
err := helper.UploadToTmp(ctx, &filePath, uploadedFile, &helper.CloudStorageTmpUpload{
    Storage:       gcsClient,
    TmpBucketName: "temp-uploads",
})
```

The `*path` is updated in-place to the final upload path.
