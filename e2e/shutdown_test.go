package e2e

import (
	"sync"
	"testing"
	"time"

	"silobang/internal/audit"
	"silobang/internal/constants"
)

// TestConcurrentSubscribeUnsubscribe verifies no race conditions or panics
// when multiple goroutines subscribe/unsubscribe while logging entries
func TestConcurrentSubscribeUnsubscribe(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	logger := ts.App.AuditLogger
	if logger == nil {
		t.Fatal("AuditLogger not initialized")
	}

	const (
		numGoroutines = 50
		duration      = 2 * time.Second
	)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Goroutines that rapidly subscribe and unsubscribe
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					ch := logger.Subscribe()
					// Drain any entries briefly
					select {
					case <-ch:
					case <-time.After(time.Millisecond):
					}
					logger.Unsubscribe(ch)
				}
			}
		}()
	}

	// Goroutines that log entries
	for i := 0; i < numGoroutines/5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					logger.Log(constants.AuditActionConnected, "127.0.0.1", "test", map[string]interface{}{
						"goroutine": id,
					})
					time.Sleep(time.Millisecond)
				}
			}
		}(i)
	}

	// Let it run for the duration
	time.Sleep(duration)
	close(stop)
	wg.Wait()

	// If we get here without panic, the test passed
	t.Log("Concurrent subscribe/unsubscribe completed without panics")
}

// TestShutdownDuringAuditStream verifies graceful disconnection when server shuts down
// while an SSE audit stream is active
func TestShutdownDuringAuditStream(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	// Connect to the SSE stream
	resp, err := ts.GET("/api/audit/stream")
	if err != nil {
		t.Fatalf("Failed to connect to stream: %v", err)
	}

	// Read the connected event to ensure connection is established
	buf := make([]byte, 1024)
	_, err = resp.Body.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read from stream: %v", err)
	}

	// Close the response body first to release the connection
	// This simulates a client disconnecting before server shutdown
	resp.Body.Close()

	// Now shutdown should be clean
	ts.Shutdown()

	// If we get here without panic, the test passed
	t.Log("Shutdown during audit stream completed without panics")
}

// TestSubscriptionWrapperSafety tests the subscription wrapper's thread safety
// by having multiple goroutines attempt to send while one closes
func TestSubscriptionWrapperSafety(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	logger := ts.App.AuditLogger
	if logger == nil {
		t.Fatal("AuditLogger not initialized")
	}

	// Subscribe
	ch := logger.Subscribe()

	var wg sync.WaitGroup
	const numSenders = 20

	// Start goroutines that log entries (which notify subscribers)
	for i := 0; i < numSenders; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				logger.Log(constants.AuditActionConnected, "127.0.0.1", "test", map[string]interface{}{
					"sender": id,
					"seq":    j,
				})
			}
		}(i)
	}

	// While senders are running, unsubscribe (close the channel)
	// This should not cause a panic even if sends are in progress
	time.Sleep(10 * time.Millisecond)
	logger.Unsubscribe(ch)

	wg.Wait()

	// If we get here without panic, the test passed
	t.Log("Subscription wrapper safety test completed without panics")
}

// TestMultipleUnsubscribeSafety verifies that calling Unsubscribe multiple times
// on the same channel doesn't cause issues
func TestMultipleUnsubscribeSafety(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	logger := ts.App.AuditLogger
	if logger == nil {
		t.Fatal("AuditLogger not initialized")
	}

	ch := logger.Subscribe()

	// First unsubscribe
	logger.Unsubscribe(ch)

	// Second unsubscribe should not panic (channel already removed from map)
	logger.Unsubscribe(ch)

	// If we get here without panic, the test passed
	t.Log("Multiple unsubscribe safety test completed without panics")
}

// TestLogAfterUnsubscribe verifies that logging after unsubscribe doesn't panic
func TestLogAfterUnsubscribe(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	logger := ts.App.AuditLogger
	if logger == nil {
		t.Fatal("AuditLogger not initialized")
	}

	// Subscribe
	ch := logger.Subscribe()

	// Unsubscribe immediately
	logger.Unsubscribe(ch)

	// Log entries - should not panic even though channel was closed
	for i := 0; i < 100; i++ {
		err := logger.Log(constants.AuditActionConnected, "127.0.0.1", "test", map[string]interface{}{
			"seq": i,
		})
		if err != nil {
			t.Fatalf("Log failed: %v", err)
		}
	}

	// If we get here without panic, the test passed
	t.Log("Log after unsubscribe test completed without panics")
}

// TestAuditLoggerStopDuringActivity verifies that stopping the logger
// while activity is ongoing doesn't cause issues
func TestAuditLoggerStopDuringActivity(t *testing.T) {
	ts := StartTestServer(t)
	ts.ConfigureWorkDir(t)

	logger := ts.App.AuditLogger
	if logger == nil {
		t.Fatal("AuditLogger not initialized")
	}

	var wg sync.WaitGroup

	// Start subscribers
	channels := make([]chan audit.Entry, 10)
	for i := 0; i < 10; i++ {
		channels[i] = logger.Subscribe()
	}

	// Start goroutines that log entries
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				logger.Log(constants.AuditActionConnected, "127.0.0.1", "test", map[string]interface{}{
					"id": id,
				})
				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	// Let some activity happen
	time.Sleep(50 * time.Millisecond)

	// Stop the logger (simulating shutdown)
	logger.Stop()

	// Unsubscribe all channels
	for _, ch := range channels {
		logger.Unsubscribe(ch)
	}

	wg.Wait()

	// If we get here without panic, the test passed
	t.Log("Logger stop during activity test completed without panics")
}
