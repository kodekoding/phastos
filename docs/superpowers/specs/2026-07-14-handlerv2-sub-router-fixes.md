# HandlerV2 on Version-Group Sub-Router: Bug Fixes

**Date:** 2026-07-14
**Context:** Event controller pilot migration to `HandlerV2` (auto-binding handler)

## Problem

`HandlerV2` routes with path parameters (`/{id}`, `/{id:int64}`) returned HTTP 405 ("method not allowed") when routed through the version-group chi sub-router (`app.Http.Mount("/v1", router)`). 

Non-path-param routes (`/event`, `/checkpoint/locations`) worked fine. Old-style `Handler` routes with path params also worked fine. The issue was specific to `HandlerV2` + path params + version-group sub-router.

## Root Cause (Two Bugs)

### Bug 1: `stripPathParamTypes` corrupted path param patterns

**File:** `go/api/controller.go` — function `stripPathParamTypes`

**Symptom:** When type annotations were present in the path (e.g., `/{id:int64}`), the resulting chi route pattern was corrupted. The route was NOT registered on the sub-router at all, causing 405 for matching requests.

**Root Cause:** The byte-arithmetic removal formula at line 350 was wrong:
```go
result = result[:len(result)-(i-colon)]
```

This removed `i - colon` bytes from the END of the result buffer. But the characters between `:` and `}` (`:int64` — 6 bytes) were NEVER added to the result buffer (they were skipped by a `continue` statement). 

So the formula removed 6 bytes from the accumulated result, which included bytes that should have been kept (part of the param name `id`):
- Input path: `/v1/event/{id:int64}`
- Accumulated result at `}`: `/v1/event/{id` (14 bytes — colon and "int64" skipped by continue)
- Removal formula: `result[:14 - 6]` = `result[:8]` = `/v1/even`
- Final result: `/v1/even/` — corrupted!

The pattern `/v1/even/` was registered with chi instead of `/v1/event/{id}`, so requests to `/v1/event/3` never matched.

**Fix:** Replaced with an in-place two-pointer algorithm that skips type annotations without corrupting adjacent bytes:

```go
func stripPathParamTypes(path string) string {
    b := []byte(path)
    w := 0
    inParam := false
    inType := false
    for r := 0; r < len(b); r++ {
        ch := b[r]
        if ch == '{' {
            inParam = true
            inType = false
        } else if inParam && ch == ':' {
            inType = true
            continue
        } else if ch == '}' && inParam {
            inParam = false
            inType = false
        }
        if inType {
            continue
        }
        b[w] = ch
        w++
    }
    return string(b[:w])
}
```

**Test added:**
- `TestStripPathParamTypes_WithInt64` — verifies `/{id:int64}` → `/{id}`
- `TestStripPathParamTypes_MultipleParams` — verifies multiple typed params
- `TestStripPathParamTypes_NoType` — verifies untyped params pass through unchanged

---

### Bug 2: Chi wildcard param `*` in Mounted sub-router

**File:** `go/api/app.go` — function `wrapHandlerV2WithMeta`

**Symptom:** After Bug 1 was fixed, path param routes matched correctly (no 405), but path param validation failed with error "path param expects int64, got ''" — the first path param's value was always empty.

**Root Cause:** Chi v5 adds a wildcard key `*` at **index 0** of `rctx.URLParams.Keys` when routing through a `Mount`ed sub-router. The `*` key has an empty value `""`.

The path param validation loop used the loop index `i` to index into `pathParamTypes`:
```go
for i, key := range rctx.URLParams.Keys {
    val := rctx.URLParams.Values[i]
    if i < len(m.pathParamTypes) {
        if err := validatePathParam(val, m.pathParamTypes[i]); err != nil {
            // FAILED: validating "" as int64 for wildcard at index 0
```

Params for a route like `/{id:int64}` with request `/v1/event/50`:
```
Chi RouteContext.URLParams:
  Keys:   ["*", "id"]
  Values: ["",  "50"]
```

At index `i=0`:
- `key = "*"`, `val = ""`
- `i=0 < len(m.pathParamTypes)` (pathParamTypes = [ParamInt64], length 1) → TRUE
- `validatePathParam("", ParamInt64)` → FAILS: "path param expects int64, got ''"

The `"`*`"` wildcard param consumed the first type-check position, leaving the actual `"id"` param at index 1 unmatched.

**Fix:** Skip chi wildcard params (keys starting with `*`) and use a separate counter `pi` for indexing into `pathParamTypes`:
```go
pi := 0
for i, key := range rctx.URLParams.Keys {
    if strings.HasPrefix(key, "*") {
        continue  // skip wildcard
    }
    val := rctx.URLParams.Values[i]
    params[key] = val
    if pi < len(m.pathParamTypes) {
        if err := validatePathParam(val, m.pathParamTypes[pi]); err != nil {
            // ... return error
        }
    }
    pi++
}
```

---

## Summary

| Bug | File | Lines | Impact |
|-----|------|-------|--------|
| stripPathParamTypes byte corruption | `controller.go` | 336-360 | Route not registered → 405 |
| Chi wildcard param indexing | `app.go` | 627-645 | First path param always validated as empty → 400 |

Both fixes are in phastos v2.52.0 (local, needs re-publish).
