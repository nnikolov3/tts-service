# TTS Service

## Project Summary

A NATS-based microservice that converts text to speech using the `chatllm` binary.

## Detailed Description

This service listens for `TextProcessedEvent` messages on a NATS stream. When a message is received, it downloads the text from a NATS object store, uses the `chatllm` binary to convert the text to speech, and then uploads the resulting audio to another NATS object store. For each generated audio file, it publishes an `AudioChunkCreatedEvent` to a NATS stream.

This service is the final stage in the document processing pipeline, converting the extracted and processed text into an audio format.

Core capabilities include:

-   **NATS Integration**: Seamlessly integrates with NATS for messaging and object storage.
-   **Text-to-Speech Conversion**: Utilizes the `chatllm` binary for high-quality text-to-speech synthesis.
-   **Robust Error Handling**: Implements `ack`, `nak`, and `term` logic for handling NATS messages.

## Technology Stack

-   **Programming Language:** Go 1.25
-   **Messaging:** NATS
-   **TTS Engine:** `chatllm`
-   **Libraries:**
    -   `github.com/nats-io/nats.go`
    -   `github.com/book-expert/configurator`
    -   `github.com/book-expert/events`
    -   `github.com/book-expert/logger`
    -   `github.com/google/uuid`
    -   `github.com/stretchr/testify`

## Getting Started

### Prerequisites

-   Go 1.25 or later.
-   NATS server with JetStream enabled.
-   The `chatllm` binary installed and available in the system's `PATH`.

### Installation

To build the service, you can use the `make build` command:

```bash
make build
```

This will create the `tts-service` binary in the `bin` directory.

### Configuration

The service requires a TOML configuration file to be accessible via a URL specified by the `PROJECT_TOML` environment variable. The configuration file should have the following structure:

```toml
[nats]
url = "nats://localhost:4222"
text_processed_subject = "text.processed"
audio_object_store_bucket = "audio_files"

[tts]
model_path = "/path/to/your/model.bin"
snac_model_path = "/path/to/your/snac_model.bin"
voice = "default"
seed = 1234
ngl = 0
top_p = 0.95
repetition_penalty = 1.1
temperature = 0.7
```

## Usage

To run the service, execute the binary:

```bash
./bin/tts-service
```

The service will connect to NATS and start listening for messages.

## Testing

To run the tests for this service, you can use the `make test` command:

```bash
make test
```

## License

Distributed under the MIT License. See the `LICENSE` file for more information.
