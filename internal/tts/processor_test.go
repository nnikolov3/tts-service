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

	_, err := tts.New("dummy/path/to/chatllm")
	require.NoError(t, err)
}

func TestChatLLMProcessor_Process(t *testing.T) {
	t.Parallel()

	processor, err := tts.New("dummy/path/to/chatllm")
	require.NoError(t, err)

	// The Process method is not implemented yet, so we expect the specific error.
	_, err = processor.Process(context.Background(), []byte("hello"))
	require.Error(t, err)
	require.Equal(t, tts.ErrNotImplemented, err)
}
