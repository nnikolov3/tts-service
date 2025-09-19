// Package worker provides a NATS worker that processes TTS jobs.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/book-expert/events"
	"github.com/book-expert/logger"
	"github.com/book-expert/tts-service/internal/core"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

const handleMessageTimeout = 30 * time.Second

// NatsWorker listens for TTS jobs on a NATS subject and processes them.
type NatsWorker struct {
	natsConnection   *nats.Conn
	jetstreamContext nats.JetStreamContext
	subject          string
	store            core.ObjectStore
	processor        core.TTSProcessor
	log              *logger.Logger
}

// NewNatsWorker creates a new instance of a NATS worker.
func NewNatsWorker(
	natsConnection *nats.Conn,
	jetstreamContext nats.JetStreamContext,
	subject string,
	store core.ObjectStore,
	processor core.TTSProcessor,
	log *logger.Logger,
) (*NatsWorker, error) {
	return &NatsWorker{
		natsConnection:   natsConnection,
		jetstreamContext: jetstreamContext,
		subject:          subject,
		store:            store,
		processor:        processor,
		log:              log,
	}, nil
}

// Run starts the worker and begins listening for messages.
func (w *NatsWorker) Run(ctx context.Context) error {
	sub, err := w.natsConnection.Subscribe(w.subject, w.handleMessage)
	if err != nil {
		return fmt.Errorf("failed to subscribe to subject %s: %w", w.subject, err)
	}

	<-ctx.Done()

	drainErr := sub.Drain()
	if drainErr != nil {
		return fmt.Errorf("failed to drain subscription: %w", drainErr)
	}

	return nil
}

func (w *NatsWorker) handleMessage(msg *nats.Msg) {
	ctx, cancel := context.WithTimeout(context.Background(), handleMessageTimeout)
	defer cancel()

	var event events.TextProcessedEvent

	err := json.Unmarshal(msg.Data, &event)
	if err != nil {
		w.log.Error("Failed to unmarshal event: %v", err)

		return
	}

	textData, err := w.store.Download(ctx, event.TextKey)
	if err != nil {
		w.log.Error("Failed to download text data for key '%s': %v", event.TextKey, err)

		return
	}

	audioData, err := w.processor.Process(ctx, textData)
	if err != nil {
		w.log.Error("Failed to process text to speech: %v", err)

		return
	}

	audioKey := uuid.NewString() + ".wav"

	err = w.store.Upload(ctx, audioKey, audioData)
	if err != nil {
		w.log.Error("Failed to upload audio data for key '%s': %v", audioKey, err)

		return
	}

	replyEvent := &events.AudioChunkCreatedEvent{
		Header:     event.Header,
		AudioKey:   audioKey,
		PageNumber: event.PageNumber,
		TotalPages: event.TotalPages,
	}

	replyData, err := json.Marshal(replyEvent)
	if err != nil {
		w.log.Error("Failed to marshal reply event: %v", err)

		return
	}

	err = msg.Respond(replyData)
	if err != nil {
		w.log.Error("Failed to publish reply event: %v", err)
	}
}
