package server

import (
	"sync"
	"testing"

	"silobang/internal/config"
	"silobang/internal/logger"
)

func newTestApp() *App {
	cfg := &config.Config{}
	log := logger.NewLogger(logger.LevelError)
	return NewApp(cfg, log)
}

func TestGetTopicWriteMu_LazyCreation(t *testing.T) {
	app := newTestApp()

	mu := app.GetTopicWriteMu("test-topic")
	if mu == nil {
		t.Fatal("GetTopicWriteMu returned nil")
	}
}

func TestGetTopicWriteMu_Idempotent(t *testing.T) {
	app := newTestApp()

	mu1 := app.GetTopicWriteMu("test-topic")
	mu2 := app.GetTopicWriteMu("test-topic")

	if mu1 != mu2 {
		t.Error("GetTopicWriteMu returned different pointers for same topic")
	}
}

func TestGetTopicWriteMu_DifferentTopics(t *testing.T) {
	app := newTestApp()

	mu1 := app.GetTopicWriteMu("topic-a")
	mu2 := app.GetTopicWriteMu("topic-b")

	if mu1 == mu2 {
		t.Error("GetTopicWriteMu returned same pointer for different topics")
	}
}

func TestGetTopicWriteMu_ConcurrentAccess(t *testing.T) {
	app := newTestApp()

	const numGoroutines = 50
	var wg sync.WaitGroup
	results := make(chan *sync.Mutex, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu := app.GetTopicWriteMu("same-topic")
			results <- mu
		}()
	}

	wg.Wait()
	close(results)

	// All goroutines should get the same pointer
	var first *sync.Mutex
	for mu := range results {
		if mu == nil {
			t.Fatal("GetTopicWriteMu returned nil")
		}
		if first == nil {
			first = mu
		} else if mu != first {
			t.Error("GetTopicWriteMu returned different pointers under concurrent access")
		}
	}
}

func TestUnregisterTopic_CleansMutex(t *testing.T) {
	app := newTestApp()

	// Register a topic and get its mutex
	app.RegisterTopic("cleanup-topic", true, "")
	mu1 := app.GetTopicWriteMu("cleanup-topic")
	if mu1 == nil {
		t.Fatal("GetTopicWriteMu returned nil before unregister")
	}

	// Unregister the topic
	app.UnregisterTopic("cleanup-topic")

	// Get mutex again - should be a fresh one
	mu2 := app.GetTopicWriteMu("cleanup-topic")
	if mu2 == nil {
		t.Fatal("GetTopicWriteMu returned nil after unregister")
	}
	if mu1 == mu2 {
		t.Error("GetTopicWriteMu returned same pointer after unregister - mutex was not cleaned up")
	}
}

func TestClearTopicRegistry_ClearsMutexes(t *testing.T) {
	app := newTestApp()

	// Create mutexes for several topics
	mu1 := app.GetTopicWriteMu("topic-1")
	mu2 := app.GetTopicWriteMu("topic-2")

	if mu1 == nil || mu2 == nil {
		t.Fatal("GetTopicWriteMu returned nil")
	}

	// Clear the registry
	app.ClearTopicRegistry()

	// Get mutexes again - should be fresh
	mu1New := app.GetTopicWriteMu("topic-1")
	mu2New := app.GetTopicWriteMu("topic-2")

	if mu1 == mu1New {
		t.Error("topic-1 mutex not cleaned up after ClearTopicRegistry")
	}
	if mu2 == mu2New {
		t.Error("topic-2 mutex not cleaned up after ClearTopicRegistry")
	}
}

func TestGetTopicCreateMu_ReturnsSameMutex(t *testing.T) {
	app := newTestApp()

	mu1 := app.GetTopicCreateMu()
	mu2 := app.GetTopicCreateMu()

	if mu1 == nil {
		t.Fatal("GetTopicCreateMu returned nil")
	}
	if mu1 != mu2 {
		t.Error("GetTopicCreateMu returned different pointers")
	}
}

func TestGetTopicCreateMu_IsLockable(t *testing.T) {
	app := newTestApp()

	mu := app.GetTopicCreateMu()

	// Should be lockable without deadlock
	mu.Lock()
	mu.Unlock()
}
