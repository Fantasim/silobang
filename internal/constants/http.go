package constants

import "time"

// HTTP Server Timeouts
const (
	HTTPIdleTimeoutSecs = 120
	HTTPIdleTimeout     = HTTPIdleTimeoutSecs * time.Second
)

// Content Types
const (
	ContentTypeJSON = "application/json"
	ContentTypeSSE  = "text/event-stream"
	ContentTypeText = "text/plain; charset=utf-8"
)

// SSE (Server-Sent Events) Headers
const (
	SSECacheControl    = "no-cache"
	SSEConnection      = "keep-alive"
	SSEXAccelBuffering = "no"
)

// Content-Disposition Headers
const (
	ContentDispositionFormat = `attachment; filename="%s"`
	BulkDownloadZipFilename  = "download.zip"
)

// Transfer Encoding
const (
	TransferEncodingChunked = "chunked"
)

// HTTP Header Names
const (
	HeaderContentType        = "Content-Type"
	HeaderContentDisposition = "Content-Disposition"
	HeaderCacheControl       = "Cache-Control"
	HeaderConnection         = "Connection"
	HeaderXAccelBuffering    = "X-Accel-Buffering"
	HeaderTransferEncoding   = "Transfer-Encoding"
)
