// Package worker_test tests the NATS worker for the TTS service.
package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/book-expert/events"
	"github.com/book-expert/logger"
	"github.com/book-expert/tts-service/internal/worker"
	"github.com/google/uuid"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	errMockDownload = errors.New("mock download error")
	errMockUpload   = errors.New("mock upload error")
	errMockProcess  = errors.New("mock process error")
)

// mockObjectStore is a mock implementation of the ObjectStore interface.
type mockObjectStore struct {
	downloadShouldFail bool
	uploadShouldFail   bool
	downloadedKey      string
	uploadedKey        string
	uploadedData       []byte
}

func (m *mockObjectStore) Download(_ context.Context, key string) ([]byte, error) {
	if m.downloadShouldFail {
		return nil, errMockDownload
	}

	m.downloadedKey = key

	return []byte("sample text"), nil
}

func (m *mockObjectStore) Upload(_ context.Context, key string, data []byte) error {
	if m.uploadShouldFail {
		return errMockUpload
	}

	m.uploadedKey = key
	m.uploadedData = data

	return nil
}

// mockTTSProcessor is a mock implementation of the TTSProcessor interface.
type mockTTSProcessor struct {
	processShouldFail bool
	processedText     []byte
}

func (m *mockTTSProcessor) Process(_ context.Context, text []byte) ([]byte, error) {
	if m.processShouldFail {
		return nil, errMockProcess
	}

	m.processedText = text

	return []byte("sample audio"), nil
}

func setupTest(t *testing.T) (
	*worker.NatsWorker,
	*mockObjectStore,
	*mockTTSProcessor,
	context.Context,
	context.CancelFunc,
	*nats.Conn,
) {
	t.Helper()

	mockStore := &mockObjectStore{
		downloadShouldFail: false,
		uploadShouldFail:   false,
		downloadedKey:      "",
		uploadedKey:        "",
		uploadedData:       nil,
	}
	mockProcessor := &mockTTSProcessor{
		processShouldFail: false,
		processedText:     nil,
	}

	server, natsConnection := StartTestServer(t)

	jetstreamContext, err := natsConnection.JetStream()
	require.NoError(t, err)

	testLogger, err := logger.New("/tmp", "test-log.log")
	require.NoError(t, err)

	workerInstance, err := worker.NewNatsWorker(
		natsConnection, jetstreamContext, "test_subject", mockStore, mockProcessor, testLogger,
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	// Important: Defer shutdown of the server using a cleanup function to ensure it runs after the test completes.
	t.Cleanup(func() {
		server.Shutdown()
		natsConnection.Close()
	})

	return workerInstance, mockStore, mockProcessor, ctx, cancel, natsConnection
}

func TestMessageHandler_Success(t *testing.T) {
	t.Parallel()

	workerInstance, mockStore, mockProcessor, ctx, cancel, natsConnection := setupTest(t)
	defer cancel()

	errChan := make(chan error, 1)

	go func() {
		errChan <- workerInstance.Run(ctx)
	}()

	testEvent := &events.TextProcessedEvent{
		Header: events.EventHeader{
			Timestamp:  time.Now(),
			WorkflowID: uuid.NewString(),
			EventID:    uuid.NewString(),
			UserID:     "",
			TenantID:   "",
		},
		TextKey:    "test-text-key",
		PNGKey:     "",
		PageNumber: 0,
		TotalPages: 0,
	}
	eventData, err := json.Marshal(testEvent)
	require.NoError(t, err)

	replyMsg, err := natsConnection.Request("test_subject", eventData, 5*time.Second)
	require.NoError(t, err, "Request should succeed and receive a reply")

	var replyEvent events.AudioChunkCreatedEvent

	err = json.Unmarshal(replyMsg.Data, &replyEvent)
	require.NoError(t, err)

	assert.Equal(t, "test-text-key", mockStore.downloadedKey)
	assert.Equal(t, []byte("sample text"), mockProcessor.processedText)
	assert.NotEmpty(t, mockStore.uploadedKey, "An audio key should have been generated and uploaded")
	assert.Equal(t, []byte("sample audio"), mockStore.uploadedData)

	assert.Equal(t, mockStore.uploadedKey, replyEvent.AudioKey)
	assert.Equal(t, testEvent.Header.WorkflowID, replyEvent.Header.WorkflowID)

	cancel()

	shutdownErr := <-errChan
	assert.NoError(t, shutdownErr, "worker.Run should not error on graceful shutdown")
}

// StartTestServer starts an in-memory NATS server for testing purposes.
func StartTestServer(t *testing.T) (*server.Server, *nats.Conn) {
	t.Helper()

	opts := test.DefaultTestOptions
	opts.Port = -1 // Use a random port
	opts.JetStream = true
	server := test.RunServer(&opts)

	natsConnection, err := nats.Connect(server.ClientURL())
	if err != nil {
		t.Fatalf("Failed to connect to test NATS server: %v", err)
	}

	return server, natsConnection
}
