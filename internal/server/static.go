package server

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"silobang/internal/constants"
)

// etagCache stores pre-computed ETags for embedded files.
// Since the embedded FS is immutable per binary, ETags are computed once per file.
var etagCache sync.Map // map[string]string

// computeETag returns a strong ETag for the given content.
// Results are cached by path since embedded content never changes.
func computeETag(path string, data []byte) string {
	if cached, ok := etagCache.Load(path); ok {
		return cached.(string)
	}
	hash := sha256.Sum256(data)
	etag := fmt.Sprintf(`"%x"`, hash[:8])
	etagCache.Store(path, etag)
	return etag
}

// serveStaticWithCompression serves embedded static files with pre-compressed
// variant support. It checks Accept-Encoding for brotli (preferred) or gzip,
// and tries to serve the corresponding .br or .gz file from the embedded FS.
// Falls back to uncompressed if no compressed variant exists.
// SPA routing: non-file paths fall back to index.html for client-side routing.
func (s *Server) serveStaticWithCompression(w http.ResponseWriter, r *http.Request) {
	reqPath := strings.TrimPrefix(r.URL.Path, "/")
	if reqPath == "" {
		reqPath = "index.html"
	}

	// Check if the requested file exists in the embedded FS
	_, err := fs.Stat(s.webFS, reqPath)
	if err != nil {
		// SPA fallback: serve index.html for client-side routing
		reqPath = "index.html"
	}

	// Determine the MIME type from the original filename (not compressed extension)
	contentType := mime.TypeByExtension(filepath.Ext(reqPath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Determine cache control based on path:
	// - Hashed assets (e.g., /assets/index-BcX3sT.js) → immutable, 1 year
	// - Everything else (index.html, favicon) → revalidate every request
	cacheControl := constants.CacheControlNoCache
	if strings.HasPrefix(reqPath, "assets/") {
		cacheControl = constants.CacheControlStaticHash
	}

	acceptEncoding := r.Header.Get("Accept-Encoding")

	// Try brotli first (smaller, preferred)
	if strings.Contains(acceptEncoding, "br") {
		if served := s.serveCompressedVariant(w, r, reqPath, constants.CompressedFileExtBrotli, "br", contentType, cacheControl); served {
			return
		}
	}

	// Try gzip
	if strings.Contains(acceptEncoding, "gzip") {
		if served := s.serveCompressedVariant(w, r, reqPath, constants.CompressedFileExtGzip, "gzip", contentType, cacheControl); served {
			return
		}
	}

	// Fall back to uncompressed
	s.serveUncompressedFile(w, r, reqPath, contentType, cacheControl)
}

// serveCompressedVariant attempts to serve a pre-compressed variant (.br or .gz)
// from the embedded FS. Returns true if the file was found and served.
// Uses the uncompressed file's ETag (based on original content) for conditional requests.
func (s *Server) serveCompressedVariant(w http.ResponseWriter, r *http.Request, reqPath, ext, encoding, contentType, cacheControl string) bool {
	compressedPath := reqPath + ext
	f, err := s.webFS.Open(compressedPath)
	if err != nil {
		return false
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		s.logger.Warn("Static: failed to read compressed file %s: %v", compressedPath, err)
		return false
	}

	// Compute ETag from compressed content (unique per encoding variant)
	etag := computeETag(compressedPath, data)

	// Check conditional request
	if r.Header.Get("If-None-Match") == etag {
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", cacheControl)
		w.WriteHeader(http.StatusNotModified)
		return true
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Encoding", encoding)
	w.Header().Set("Vary", "Accept-Encoding")
	w.Header().Set("Cache-Control", cacheControl)
	w.Header().Set("ETag", etag)
	w.Write(data) //nolint:errcheck
	return true
}

// serveUncompressedFile serves the original uncompressed file from the embedded FS.
func (s *Server) serveUncompressedFile(w http.ResponseWriter, r *http.Request, reqPath, contentType, cacheControl string) {
	f, err := s.webFS.Open(reqPath)
	if err != nil {
		http.NotFound(w, nil)
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		s.logger.Error("Static: failed to read file %s: %v", reqPath, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Compute ETag from content
	etag := computeETag(reqPath, data)

	// Check conditional request
	if r.Header.Get("If-None-Match") == etag {
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", cacheControl)
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", cacheControl)
	w.Header().Set("ETag", etag)
	w.Write(data) //nolint:errcheck
}
