package asynclog

import (
	"context"
	"fmt"
	"os"
	"sync"
)

// FileSink writes log messages to a file in JSON format
type FileSink struct {
	filePath string
	file     *os.File
	mu       sync.Mutex
}

// NewFileSink creates a new file sink
func NewFileSink(filePath string) (*FileSink, error) {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}

	return &FileSink{
		filePath: filePath,
		file:     file,
	}, nil
}

// Write implements the Sink interface
func (fs *FileSink) Write(ctx context.Context, messages []*LogMessage) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	for _, msg := range messages {
		jsonBytes, err := msg.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to marshal message to JSON: %w", err)
		}
		
		if _, err := fmt.Fprintf(fs.file, "%s\n", jsonBytes); err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
	}

	// Sync to disk
	return fs.file.Sync()
}

// Close implements the Sink interface
func (fs *FileSink) Close() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.file != nil {
		err := fs.file.Close()
		fs.file = nil
		return err
	}
	return nil
}

// Name implements the Sink interface
func (fs *FileSink) Name() string {
	return fmt.Sprintf("file(%s)", fs.filePath)
}