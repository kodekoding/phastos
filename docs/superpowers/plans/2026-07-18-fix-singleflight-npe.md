# Fix Singleflight NPE in wrapHandler

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix NPE in `app.sf.Do` inside `wrapHandler` yang terjadi saat handler panic atau return nil `*Response` ketika singleflight aktif.

**Architecture:** Dua akar masalah: (1) `singleflight.Do` re-panic ke semua duplicate caller saat handler pertama panic, (2) `response.TraceId` NPE kalau channel kosong setelah goroutine exit. Fix: bungkus handler call di `sf.Do` dengan defer-recover + nil-check response.

**Tech Stack:** Go 1.26, `golang.org/x/sync/singleflight`, `net/http/httptest`

**Spec:** [2026-07-18-fix-singleflight-npe-design.md](brainstorming session — no separate spec file)

## Global Constraints

- Hanya ubah `go/api/app.go` (wrapHandler) dan `go/api/wraphandler_test.go` (tests)
- Tidak boleh ubah public API/interface
- Tidak boleh ubah `go/api/fasthttp_app.go` (tidak pakai singleflight)
- Test coverage harus bertambah (tidak boleh turun)
- Ikuti test pattern yang sudah ada: `os.Setenv("SINGLEFLIGHT_ACTIVE", "true")`, httptest.NewRecorder

---

## File Structure

| File | Role |
|------|------|
| `go/api/app.go:553-578` | Fix goroutine di wrapHandler — recover di sf.Do closure + nil-safety |
| `go/api/wraphandler_test.go` | 3 test baru: handler panic, handler nil return, concurrent panic |

---

### Task 1: Fix goroutine in wrapHandler — recover + nil-safety

**Files:**
- Modify: `go/api/app.go:553-578`

**Interfaces:**
- Produces: goroutine in `wrapHandler` that never panics and always sends to `respChan`

- [ ] **Step 1: Write failing tests**

Add to `go/api/wraphandler_test.go` — 3 tests that should pass after the fix:

```go

// --- wrapHandler async path: singleflight + handler panic ---

func TestApp_WrapHandler_AsyncPath_Singleflight_HandlerPanic(t *testing.T) {
	originalSF := os.Getenv("SINGLEFLIGHT_ACTIVE")
	os.Setenv("SINGLEFLIGHT_ACTIVE", "true")
	defer os.Setenv("SINGLEFLIGHT_ACTIVE", originalSF)

	app := NewApp(WithTimezone("UTC"), WithAPITimeout(5))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		panic("simulated panic in handler")
	}

	wrapped := app.wrapHandler(handler)
	req := httptest.NewRequest(http.MethodGet, "/sf-panic", nil)
	req.Header.Set(common.RequestIDHeader, "sf-panic-trace")
	req.Header.Set("X-Forwarded-For", "10.0.0.3")
	w := httptest.NewRecorder()
	wrapped(w, req)

	// Must not crash. Should return error response.
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- wrapHandler async path: singleflight + handler returns nil ---

func TestApp_WrapHandler_AsyncPath_Singleflight_HandlerNilReturn(t *testing.T) {
	originalSF := os.Getenv("SINGLEFLIGHT_ACTIVE")
	os.Setenv("SINGLEFLIGHT_ACTIVE", "true")
	defer os.Setenv("SINGLEFLIGHT_ACTIVE", originalSF)

	app := NewApp(WithTimezone("UTC"), WithAPITimeout(5))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		return nil
	}

	wrapped := app.wrapHandler(handler)
	req := httptest.NewRequest(http.MethodGet, "/sf-nil", nil)
	req.Header.Set(common.RequestIDHeader, "sf-nil-trace")
	req.Header.Set("X-Forwarded-For", "10.0.0.4")
	w := httptest.NewRecorder()
	wrapped(w, req)

	// Must not NPE. Should return error response.
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- wrapHandler async path: singleflight + concurrent handler panic ---

func TestApp_WrapHandler_AsyncPath_Singleflight_ConcurrentPanic(t *testing.T) {
	originalSF := os.Getenv("SINGLEFLIGHT_ACTIVE")
	os.Setenv("SINGLEFLIGHT_ACTIVE", "true")
	defer os.Setenv("SINGLEFLIGHT_ACTIVE", originalSF)

	app := NewApp(WithTimezone("UTC"), WithAPITimeout(5))
	app.Init()

	var callCount atomic.Int32
	handler := func(req Request, ctx context.Context) *Response {
		callCount.Add(1)
		panic("concurrent panic")
	}

	wrapped := app.wrapHandler(handler)

	var wg sync.WaitGroup
	const n = 10
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/sf-panic", nil)
			req.Header.Set(common.RequestIDHeader, "sf-conc-trace")
			req.Header.Set("X-Forwarded-For", "10.0.0.5")
			w := httptest.NewRecorder()
			// Must not crash any goroutine
			wrapped(w, req)
			assert.Equal(t, http.StatusInternalServerError, w.Code)
		}()
	}
	wg.Wait()

	// With singleflight, handler should be called fewer than n times
	// (deduplication). But since it panics, all duplicate calls share
	// the panic-error. Still, singleflight should limit calls.
	assert.LessOrEqual(t, callCount.Load(), int32(n))
}
```

- [ ] **Step 2: Run tests to verify they FAIL**

```bash
go test ./go/api/ -run 'Singleflight_HandlerPanic|Singleflight_HandlerNilReturn|Singleflight_ConcurrentPanic' -v -count=1
```

Expected: at least one test panics with NPE

- [ ] **Step 3: Fix goroutine in wrapHandler**

In `go/api/app.go`, replace lines 553-578 (the goroutine inside `wrapHandler`) with:

```go
		go func() {
			var resp *Response
			defer func() {
				// Always send a response before closing the channel.
				// Guards against: handler panic caught by singleflight re-panic,
				// and handler returning nil *Response.
				if resp == nil {
					resp = NewResponse().SetError(InternalServerError("internal error", "INTERNAL_ERROR"))
				}
				respChan <- resp
				ReleaseRequest(request)
				close(respChan)
			}()
			var uniqueReqKey string
			defer panicRecover(r, requestId, uniqueReqKey)

			if !app.sfActive {
				resp = h(*request, ctx)
				return
			}

			uniqueReqKey = generateUniqueRequestKey(r)

			sfResponse, sfErr, _ := app.sf.Do(uniqueReqKey, func() (any, error) {
				defer func() {
					if r := recover(); r != nil {
						log.Error().Interface("panic", r).Msg("[SINGLEFLIGHT] handler panic recovered")
					}
				}()
				handlerResp := h(*request, ctx)
				return handlerResp, nil
			})
			if sfErr != nil {
				log.Err(sfErr).Msg("[SINGLEFLIGHT] - Error when do singleFlight request")
				resp = NewResponse().SetError(sfErr)
				return
			}
			resp, _ = sfResponse.(*Response)
		}()
```

Key changes:
1. Local `var resp *Response` — accumulate result, send once at end
2. `defer` at top — always sends `resp` (or error fallback) before closing channel
3. `defer/recover` inside `sf.Do` closure — prevents handler panics from reaching singleflight's re-panic mechanism
4. Nil-safe: if `sfResponse.(*Response)` is nil, `resp` stays nil, top-level defer creates error response

- [ ] **Step 4: Run all tests to verify they PASS**

```bash
go test ./go/api/... -v -count=1 2>&1 | tail -30
```

Expected: ALL PASS, no panics

- [ ] **Step 5: Run with race detector**

```bash
go test -race ./go/api/ -run 'Singleflight' -count=1
```

Expected: PASS, no race detected

- [ ] **Step 6: Build full project**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add go/api/app.go go/api/wraphandler_test.go
git commit -m "fix(api): prevent NPE when handler panics inside singleflight.Do"
```
