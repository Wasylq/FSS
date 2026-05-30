package scraper

import (
	"sync"
	"testing"
)

func TestSetVerboseAndDebugfGating(t *testing.T) {
	old := verboseLevel.Load()
	t.Cleanup(func() { verboseLevel.Store(old) })

	SetVerbose(0)
	if Verbose() != 0 {
		t.Fatalf("Verbose() = %d after SetVerbose(0)", Verbose())
	}

	SetVerbose(2)
	if Verbose() != 2 {
		t.Fatalf("Verbose() = %d after SetVerbose(2)", Verbose())
	}

	// Debugf at level 3 should not panic even when verbosity is 2.
	Debugf(3, "should not appear: %s", "test")

	// Debugf at level 1 should not panic when verbosity is 2.
	Debugf(1, "should appear: %s", "test")
}

func TestSetVerboseConcurrent(t *testing.T) {
	old := verboseLevel.Load()
	t.Cleanup(func() { verboseLevel.Store(old) })

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			SetVerbose(i % 4)
			Debugf(1, "concurrent write %d", i)
			_ = Verbose()
		}()
	}
	wg.Wait()
}
