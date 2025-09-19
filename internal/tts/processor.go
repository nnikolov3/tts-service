// Package tts provides the implementation for the TTSProcessor interface.
package tts

import (
	"context"
	"errors"
)

// ErrNotImplemented is returned when a method is not yet implemented.
var ErrNotImplemented = errors.New("not yet implemented")

// ChatLLMProcessor implements the core.TTSProcessor interface by calling the chatllm binary.
type ChatLLMProcessor struct {
	binaryPath string
}

// New creates a new ChatLLMProcessor.
func New(binaryPath string) (*ChatLLMProcessor, error) {
	return &ChatLLMProcessor{
		binaryPath: binaryPath,
	}, nil
}

// Process takes text and returns the raw audio data by calling the chatllm binary.
func (p *ChatLLMProcessor) Process(_ context.Context, _ []byte) ([]byte, error) {
	return nil, ErrNotImplemented
}
