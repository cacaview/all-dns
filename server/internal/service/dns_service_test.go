package service

import (
	"testing"
)

func TestWatchOptions_Defaults(t *testing.T) {
	opts := WatchOptions{}
	if opts.IntervalSecs != 0 {
		t.Errorf("default IntervalSecs should be 0, got %d", opts.IntervalSecs)
	}
	if opts.MaxAttempts != 0 {
		t.Errorf("default MaxAttempts should be 0, got %d", opts.MaxAttempts)
	}
	if len(opts.Resolvers) != 0 {
		t.Errorf("default Resolvers should be empty (service defaults apply), got %d", len(opts.Resolvers))
	}
}

func TestPropagationService_WatchOptionsApplied(t *testing.T) {
	opts := WatchOptions{
		Resolvers:    []string{"1.1.1.1:53", "8.8.8.8:53"},
		IntervalSecs: 60,
		MaxAttempts:  10,
	}

	if len(opts.Resolvers) != 2 {
		t.Errorf("expected 2 resolvers, got %d", len(opts.Resolvers))
	}
	if opts.IntervalSecs != 60 {
		t.Errorf("expected interval 60, got %d", opts.IntervalSecs)
	}
	if opts.MaxAttempts != 10 {
		t.Errorf("expected maxAttempts 10, got %d", opts.MaxAttempts)
	}
}
