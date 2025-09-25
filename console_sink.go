package asynclog

import (
	"context"
	"fmt"
	"os"
)

// ConsoleSink writes log messages to stdout/stderr
type ConsoleSink struct {
	useStderr bool
}

// NewConsoleSink creates a new console sink
func NewConsoleSink(useStderr bool) *ConsoleSink {
	return &ConsoleSink{
		useStderr: useStderr,
	}
}

// Write implements the Sink interface
func (cs *ConsoleSink) Write(ctx context.Context, messages []*LogMessage) error {
	output := os.Stdout
	if cs.useStderr {
		output = os.Stderr
	}

	for _, msg := range messages {
		jsonBytes, err := msg.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to marshal message to JSON: %w", err)
		}
		
		if _, err := fmt.Fprintf(output, "%s\n", jsonBytes); err != nil {
			return fmt.Errorf("failed to write to console: %w", err)
		}
	}

	return nil
}

// Close implements the Sink interface
func (cs *ConsoleSink) Close() error {
	// Nothing to close for console
	return nil
}

// Name implements the Sink interface
func (cs *ConsoleSink) Name() string {
	if cs.useStderr {
		return "console(stderr)"
	}
	return "console(stdout)"
}