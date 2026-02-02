package models

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type WorkflowLogger interface {
	Close() error
	DataWriter(idx int, stream string) io.Writer
	ControlWriter(idx int, step Step, stepStatus StepStatus) io.Writer
}

type NullLogger struct{}

func (l NullLogger) Close() error                                { return nil }
func (l NullLogger) DataWriter(idx int, stream string) io.Writer { return io.Discard }
func (l NullLogger) ControlWriter(idx int, step Step, stepStatus StepStatus) io.Writer {
	return io.Discard
}

type FileWorkflowLogger struct {
	file    *os.File
	encoder *json.Encoder
	mask    *SecretMask
}

func NewFileWorkflowLogger(baseDir string, wid WorkflowId, secretValues []string) (WorkflowLogger, error) {
	path := LogFilePath(baseDir, wid)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("creating log file: %w", err)
	}
	return &FileWorkflowLogger{
		file:    file,
		encoder: json.NewEncoder(file),
		mask:    NewSecretMask(secretValues),
	}, nil
}

func LogFilePath(baseDir string, workflowID WorkflowId) string {
	logFilePath := filepath.Join(baseDir, fmt.Sprintf("%s.log", workflowID.String()))
	return logFilePath
}

func (l *FileWorkflowLogger) Close() error {
	return l.file.Close()
}

func (l *FileWorkflowLogger) DataWriter(idx int, stream string) io.Writer {
	return &dataWriter{
		logger: l,
		idx:    idx,
		stream: stream,
	}
}

func (l *FileWorkflowLogger) ControlWriter(idx int, step Step, stepStatus StepStatus) io.Writer {
	return &controlWriter{
		logger:     l,
		idx:        idx,
		step:       step,
		stepStatus: stepStatus,
	}
}

type dataWriter struct {
	logger *FileWorkflowLogger
	idx    int
	stream string
}

func (w *dataWriter) Write(p []byte) (int, error) {
	line := strings.TrimRight(string(p), "\r\n")
	if w.logger.mask != nil {
		line = w.logger.mask.Mask(line)
	}
	entry := NewDataLogLine(w.idx, line, w.stream)
	if err := w.logger.encoder.Encode(entry); err != nil {
		return 0, err
	}
	return len(p), nil
}

type controlWriter struct {
	logger     *FileWorkflowLogger
	idx        int
	step       Step
	stepStatus StepStatus
}

func (w *controlWriter) Write(_ []byte) (int, error) {
	entry := NewControlLogLine(w.idx, w.step, w.stepStatus)
	if err := w.logger.encoder.Encode(entry); err != nil {
		return 0, err
	}
	return len(w.step.Name()), nil
}
