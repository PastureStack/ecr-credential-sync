package main

import (
	"io"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
)

type capturedLogEntries struct {
	messages chan string
}

func (hook *capturedLogEntries) Levels() []log.Level {
	return log.AllLevels
}

func (hook *capturedLogEntries) Fire(entry *log.Entry) error {
	hook.messages <- entry.Message
	return nil
}

func TestVendoredLogrusWriterHandlesLargeLines(t *testing.T) {
	logger := log.New()
	logger.Out = io.Discard
	writer := logger.Writer()
	defer writer.Close()

	payload := []byte(strings.Repeat("x", 128*1024))
	for attempt := 1; attempt <= 2; attempt++ {
		written, err := writer.Write(payload)
		if err != nil {
			t.Fatalf("large-line write %d failed: %v", attempt, err)
		}
		if written != len(payload) {
			t.Fatalf("large-line write %d wrote %d bytes; want %d", attempt, written, len(payload))
		}
	}
}

func TestVendoredLogrusWriterPreservesLineBoundaries(t *testing.T) {
	logger := log.New()
	logger.Out = io.Discard
	hook := &capturedLogEntries{messages: make(chan string, 2)}
	logger.Hooks.Add(hook)

	writer := logger.Writer()
	if _, err := io.WriteString(writer, "first\r\nsecond\n"); err != nil {
		t.Fatalf("write line-delimited payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	for _, expected := range []string{"first", "second"} {
		select {
		case actual := <-hook.messages:
			if actual != expected {
				t.Fatalf("captured message %q; want %q", actual, expected)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for message %q", expected)
		}
	}
}
