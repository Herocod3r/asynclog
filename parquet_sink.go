package asynclog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/parquet-go/parquet-go"
)

// ParquetLogRecord represents a log message in parquet format
type ParquetLogRecord struct {
	Timestamp int64  `parquet:"timestamp,optional"`
	Level     string `parquet:"level,optional"`
	Message   string `parquet:"message,optional"`
	FieldsJSON string `parquet:"fields_json,optional"`
}

// ParquetSink writes log messages to parquet files
type ParquetSink struct {
	filePath     string
	rotateSize   int64  // Max file size before rotation (bytes)
	rotateTime   time.Duration // Max time before rotation
	mu           sync.Mutex
	currentFile  *os.File
	currentWriter *parquet.Writer
	schema        *parquet.Schema
	currentSize  int64
	createdAt    time.Time
	fileCount    int
}

// ParquetSinkConfig holds configuration for the parquet sink
type ParquetSinkConfig struct {
	// FilePath is the base path for parquet files
	FilePath string
	// RotateSize is the maximum file size in bytes before rotation (0 = no size rotation)
	RotateSize int64
	// RotateTime is the maximum duration before file rotation (0 = no time rotation)
	RotateTime time.Duration
}

// NewParquetSink creates a new parquet file sink
func NewParquetSink(config ParquetSinkConfig) (*ParquetSink, error) {
	if config.FilePath == "" {
		return nil, fmt.Errorf("file path is required")
	}

	// Ensure directory exists
	dir := filepath.Dir(config.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	sink := &ParquetSink{
		filePath:   config.FilePath,
		rotateSize: config.RotateSize,
		rotateTime: config.RotateTime,
		createdAt:  time.Now(),
	}

	// Create schema for ParquetLogRecord
	sink.schema = parquet.SchemaOf(ParquetLogRecord{})

	if err := sink.openFile(); err != nil {
		return nil, err
	}

	return sink, nil
}

// Write implements the Sink interface
func (ps *ParquetSink) Write(ctx context.Context, messages []*LogMessage) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Check if rotation is needed
	if ps.needsRotation() {
		if err := ps.rotate(); err != nil {
			return fmt.Errorf("failed to rotate file: %w", err)
		}
	}

	// Convert messages to parquet rows
	rows := make([]parquet.Row, len(messages))
	for i, msg := range messages {
		record := ps.convertToParquetRecord(msg)
		row := ps.schema.Deconstruct(nil, record)
		rows[i] = row
	}

	// Write rows
	_, err := ps.currentWriter.WriteRows(rows)
	if err != nil {
		return fmt.Errorf("failed to write to parquet file: %w", err)
	}

	// Update file size (approximate)
	ps.currentSize += int64(len(rows) * 100) // Rough estimate

	return nil
}

// Close implements the Sink interface
func (ps *ParquetSink) Close() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.currentWriter != nil {
		if err := ps.currentWriter.Close(); err != nil {
			return fmt.Errorf("failed to close parquet writer: %w", err)
		}
		ps.currentWriter = nil
	}

	if ps.currentFile != nil {
		if err := ps.currentFile.Close(); err != nil {
			return fmt.Errorf("failed to close file: %w", err)
		}
		ps.currentFile = nil
	}

	return nil
}

// Name implements the Sink interface
func (ps *ParquetSink) Name() string {
	return fmt.Sprintf("parquet(%s)", ps.filePath)
}

// needsRotation checks if file rotation is needed
func (ps *ParquetSink) needsRotation() bool {
	if ps.rotateSize > 0 && ps.currentSize >= ps.rotateSize {
		return true
	}
	if ps.rotateTime > 0 && time.Since(ps.createdAt) >= ps.rotateTime {
		return true
	}
	return false
}

// rotate closes current file and opens a new one
func (ps *ParquetSink) rotate() error {
	// Close current writer and file
	if ps.currentWriter != nil {
		if err := ps.currentWriter.Close(); err != nil {
			return err
		}
		ps.currentWriter = nil
	}
	if ps.currentFile != nil {
		if err := ps.currentFile.Close(); err != nil {
			return err
		}
		ps.currentFile = nil
	}

	// Open new file
	return ps.openFile()
}

// openFile creates a new parquet file and writer
func (ps *ParquetSink) openFile() error {
	// Generate filename with timestamp and counter
	ps.fileCount++
	timestamp := time.Now().Format("20060102_150405")
	fileName := fmt.Sprintf("%s_%s_%03d.parquet", 
		ps.filePath, timestamp, ps.fileCount)

	file, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", fileName, err)
	}

	writer := parquet.NewWriter(file, ps.schema)

	ps.currentFile = file
	ps.currentWriter = writer
	ps.currentSize = 0
	ps.createdAt = time.Now()

	return nil
}

// convertToParquetRecord converts a LogMessage to ParquetLogRecord
func (ps *ParquetSink) convertToParquetRecord(msg *LogMessage) ParquetLogRecord {
	record := ParquetLogRecord{
		Timestamp: msg.Timestamp.UnixMilli(),
		Level:     msg.Level.String(),
		Message:   msg.Message,
	}

	// Convert fields to JSON string
	if len(msg.Fields) > 0 {
		if fieldsJSON, err := json.Marshal(msg.Fields); err == nil {
			record.FieldsJSON = string(fieldsJSON)
		}
	}

	return record
}