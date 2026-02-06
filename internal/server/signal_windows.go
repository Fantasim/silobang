//go:build windows

package server

import "os"

// shutdownSignals are the OS signals that trigger graceful shutdown.
// Windows does not support SIGTERM; only Interrupt (Ctrl+C) is available.
var shutdownSignals = []os.Signal{os.Interrupt}
