package ui

import (
	"errors"
	"testing"
)

var errWriteFailed = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errWriteFailed
}

func TestPrintMiseBootstrapExplanationReturnsWriterError(t *testing.T) {
	err := PrintMiseBootstrapExplanation(failingWriter{}, "/tmp/mise")
	if !errors.Is(err, errWriteFailed) {
		t.Fatalf("error = %v, want writer error", err)
	}
}

func TestPrintStubReturnsWriterError(t *testing.T) {
	err := PrintStub(failingWriter{}, "setup shell", "not implemented")
	if !errors.Is(err, errWriteFailed) {
		t.Fatalf("error = %v, want writer error", err)
	}
}
