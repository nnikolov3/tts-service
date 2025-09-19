// Package tts_test tests the TTSProcessor implementation.
package tts_test

import (
	"context"
	"testing"

	"github.com/book-expert/tts-service/internal/tts"
	"github.com/stretchr/testify/require"
)

func TestNewChatLLMProcessor(t *testing.T) {
	t.Parallel()

	cfg := tts.Config{
		BinaryPath:    "dummy/path/to/chatllm",
		ModelPath:     "",
		SnacModelPath: "",
		Voice:         "",
	}
	_, err := tts.New(cfg)
	require.NoError(t, err)
}

func TestChatLLMProcessor_Process(t *testing.T) {
	t.Parallel()

	cfg := tts.Config{
		BinaryPath:    "dummy/path/to/chatllm",
		ModelPath:     "",
		SnacModelPath: "",
		Voice:         "",
	}
	processor, err := tts.New(cfg)
	require.NoError(t, err)

	// The Process method is not implemented yet, so we expect the specific error.
	_, err = processor.Process(context.Background(), []byte("hello"))
	require.Error(t, err)
	require.Equal(t, tts.ErrNotImplemented, err)
}
