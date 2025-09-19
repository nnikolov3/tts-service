// Package core defines the core business logic and interfaces for the TTS service.
package core

import "context"

// ObjectStore defines the interface for interacting with a key-value blob store.
type ObjectStore interface {
	Download(ctx context.Context, key string) ([]byte, error)
	Upload(ctx context.Context, key string, data []byte) error
}

// TTSProcessor defines the interface for a text-to-speech processing engine.
type TTSProcessor interface {
	Process(ctx context.Context, text []byte) ([]byte, error)
}
