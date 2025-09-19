// Package tts_test tests the TTSProcessor implementation.
package tts_test

import (
	"context"
	"testing"

	"github.com/book-expert/logger"
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
	testLogger, err := logger.New("/tmp", "test-log.log")
	require.NoError(t, err)

	_, err = tts.New(cfg, testLogger)
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
	testLogger, err := logger.New("/tmp", "test-log.log")
	require.NoError(t, err)

	processor, err := tts.New(cfg, testLogger)
	require.NoError(t, err)

	// The Process method will fail because the dummy binary path doesn't exist.
	// We just check that it returns any error.
	_, err = processor.Process(context.Background(), []byte("hello"))
	require.Error(t, err)
}
