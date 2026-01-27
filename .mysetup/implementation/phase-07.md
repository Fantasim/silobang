# Phase 7: Static File Serving

## Objective
Embed the frontend files into the Go binary using `embed.FS` and serve them from the HTTP server. Configure SPA routing so all non-API routes serve the main index.html.

---

## Prerequisites
- Phase 4 completed (HTTP server setup)
- Phase 6 completed (frontend files exist in `web/`)

---

## Task 1: Embed Directive (`internal/server/embed.go`)

Create a new file to hold the embed directive:

```go
package server

import "embed"

//go:embed all:../../web/static
var staticFS embed.FS

//go:embed ../../web/templates/index.html
var indexHTML []byte
```

**Note:** Paths are relative to the Go file location. Adjust based on final file placement.

Alternative approach - embed at cmd level:

```go
// cmd/meshbank/main.go or cmd/meshbank/embed.go

package main

import "embed"

//go:embed all:web/static
var staticFS embed.FS

//go:embed web/templates/index.html
var indexHTML []byte
```

---

## Task 2: Update Server Routes (`internal/server/server.go`)

### 2.1 Serve Static Files

Update `registerRoutes` to serve static assets:

```go
func (s *Server) registerRoutes(mux *http.ServeMux) {
    // API routes (existing)
    mux.HandleFunc("/api/config", s.handleConfig)
    mux.HandleFunc("/api/topics", s.handleTopics)
    mux.HandleFunc("/api/topics/", s.handleTopicRoutes)
    mux.HandleFunc("/api/assets/", s.handleAssetRoutes)
    mux.HandleFunc("/api/queries", s.handleQueries)
    mux.HandleFunc("/api/query/", s.handleQueryExecution)
    mux.HandleFunc("/api/logs", s.handleLogs)

    // Static files
    staticHandler := http.FileServer(http.FS(staticFS))
    mux.Handle("/static/", staticHandler)

    // SPA fallback - serve index.html for all other routes
    mux.HandleFunc("/", s.handleSPA)
}
```

### 2.2 SPA Handler

```go
// handleSPA serves index.html for all non-API, non-static routes
func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
    // If path starts with /api/ or /static/, this shouldn't be reached
    // but handle defensively
    if strings.HasPrefix(r.URL.Path, "/api/") {
        http.NotFound(w, r)
        return
    }

    // Serve index.html with correct content type
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.Write(indexHTML)
}
```

---

## Task 3: File System Wrapper

If needed, create a wrapper to strip prefix from embedded paths:

```go
// stripPrefixFS wraps an fs.FS to strip a path prefix
type stripPrefixFS struct {
    fs     fs.FS
    prefix string
}

func (s stripPrefixFS) Open(name string) (fs.File, error) {
    return s.fs.Open(path.Join(s.prefix, name))
}

// Usage:
// staticFS := stripPrefixFS{fs: embeddedFS, prefix: "web/static"}
```

---

## Task 4: Content Types

Ensure proper Content-Type headers for all file types:

| Extension | Content-Type |
|-----------|--------------|
| `.html` | `text/html; charset=utf-8` |
| `.css` | `text/css; charset=utf-8` |
| `.js` | `application/javascript; charset=utf-8` |
| `.json` | `application/json` |
| `.png` | `image/png` |
| `.svg` | `image/svg+xml` |
| `.ico` | `image/x-icon` |

Go's `http.FileServer` handles most of these automatically via `mime` package.

---

## Task 5: Caching Headers (Optional for v1)

For production, consider adding cache headers:

```go
func (s *Server) staticWithCache(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Cache static assets for 1 hour
        w.Header().Set("Cache-Control", "public, max-age=3600")
        next.ServeHTTP(w, r)
    })
}
```

For v1, skip caching - simpler debugging.

---

## Task 6: Development Mode (Optional)

For easier frontend development, optionally serve from filesystem instead of embed:

```go
var devMode = os.Getenv("MESHBANK_DEV") == "1"

func getStaticFS() fs.FS {
    if devMode {
        return os.DirFS("web/static")
    }
    return staticFS
}
```

This allows editing frontend files without rebuilding the binary.

---

## Verification Checklist

After completing Phase 7, verify:

1. **Binary contains frontend:**
   - Build binary: `go build ./cmd/meshbank`
   - Move binary to empty directory
   - Run binary - should serve frontend

2. **Static files served:**
   - `GET /static/css/tailwind.min.css` returns CSS
   - `GET /static/js/app.js` returns JavaScript
   - Correct Content-Type headers

3. **SPA routing:**
   - `GET /` returns index.html
   - `GET /topic/test` returns index.html
   - `GET /query` returns index.html
   - `GET /any/random/path` returns index.html

4. **API still works:**
   - `GET /api/config` returns JSON (not index.html)
   - `POST /api/topics` works correctly
   - No interference between static and API routes

5. **404 handling:**
   - `GET /static/nonexistent.js` returns 404
   - `GET /api/nonexistent` returns 404

---

## Files to Create/Update

| File | Action | Description |
|------|--------|-------------|
| `internal/server/embed.go` | Create | Embed directives for frontend files |
| `internal/server/server.go` | Update | Add static file serving and SPA handler |

---

## Notes

- `embed.FS` is read-only and immutable after compilation
- Embedded files increase binary size (Tailwind CSS is ~3MB minified)
- Consider using purged/minified Tailwind for smaller size
- Test with `go build` not `go run` - embed behavior differs
- Paths in embed directive are relative to the Go source file
- Use `//go:embed all:dir` to include dotfiles if needed
