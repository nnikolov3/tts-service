// Package objectstore provides a NATS-based implementation of the ObjectStore interface.
package objectstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// NatsObjectStore implements the core.ObjectStore interface using NATS JetStream.
type NatsObjectStore struct {
	jetstreamContext nats.JetStreamContext
	bucket           string
	store            nats.ObjectStore
}

// New creates and initializes a new NatsObjectStore.
func New(jetstreamContext nats.JetStreamContext, bucketName string) (*NatsObjectStore, error) {
	// Use a "create-first" approach.
	store, err := jetstreamContext.CreateObjectStore(&nats.ObjectStoreConfig{
		Bucket:      bucketName,
		Description: fmt.Sprintf("Storage for the %s bucket.", bucketName),
		TTL:         0,
		MaxBytes:    0,
		Storage:     nats.FileStorage,
		Replicas:    1,
		Placement:   nil,
		Metadata:    nil,
		Compression: false,
	})

	// If the bucket already exists, bind to it.
	if err != nil {
		if errors.Is(err, jetstream.ErrBucketExists) {
			store, err = jetstreamContext.ObjectStore(bucketName)
			if err != nil {
				return nil, fmt.Errorf("failed to bind to existing object store bucket '%s': %w", bucketName, err)
			}
		} else {
			// For any other error, fail.
			return nil, fmt.Errorf("failed to create object store bucket '%s': %w", bucketName, err)
		}
	}

	return &NatsObjectStore{
		jetstreamContext: jetstreamContext,
		bucket:           bucketName,
		store:            store,
	}, nil
}

// Download retrieves an object from the NATS object store.
func (n *NatsObjectStore) Download(_ context.Context, key string) ([]byte, error) {
	obj, err := n.store.Get(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get object '%s' from bucket '%s': %w", key, n.bucket, err)
	}

	data, readErr := io.ReadAll(obj)
	closeErr := obj.Close()

	if readErr != nil {
		return nil, fmt.Errorf("failed to read object '%s': %w", key, readErr)
	}

	if closeErr != nil {
		return data, fmt.Errorf("failed to close object '%s': %w", key, closeErr)
	}

	return data, nil
}

// Upload saves an object to the NATS object store.
func (n *NatsObjectStore) Upload(_ context.Context, key string, data []byte) error {
	reader := bytes.NewReader(data)

	_, err := n.store.Put(&nats.ObjectMeta{
		Name:        key,
		Description: "",
		Headers:     nil,
		Metadata:    nil,
		Opts:        nil,
	}, reader)
	if err != nil {
		return fmt.Errorf("failed to put object '%s' to bucket '%s': %w", key, n.bucket, err)
	}

	return nil
}
