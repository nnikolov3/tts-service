# TTS Microservice

A text-to-speech system with HTTP-based microservice architecture, implementing explicit interfaces and separation of concerns.

## Architecture

**Client-Server Model:**
- **Go Client** (`cmd/go-client`): Command-line interface for text-to-speech operations
- **Python Service** (`cmd/tts-service`): FastAPI HTTP service using OuteTTS with llama.cpp backend

**Core Components:**
- `internal/config`: Configuration management with TOML support
- `internal/tts`: HTTP client and engine implementations
- Models: OuteTTS quantized models (1B parameters, Q8_0/FP16)

## Quick Start

### 1. Setup
```bash
# Install dependencies
go mod download
pip install -r cmd/tts-service/requirements.txt

# Build client
make build
```

### 2. Start Service
```bash
# Using script (recommended)
./scripts/start_service.sh Llama-OuteTTS-1.0-1B-Q8_0.gguf

# Or directly
cd cmd/tts-service
python main.py ../../models/Llama-OuteTTS-1.0-1B-Q8_0.gguf
```

### 3. Generate Speech
```bash
# Single text
./build/tts-client --text "Hello world" --output output/hello.wav

# Batch processing
./build/tts-client --chunks input/chunks.json --output-dir output/
```

## Configuration

Edit `project.toml` for system settings:

```toml
[tts]
model_path = "models/Llama-OuteTTS-1.0-1B-Q8_0.gguf"
service_host = "127.0.0.1"
service_port = 8000
workers = 4
temperature = 0.75

[paths]
input_dir = "input"
output_dir = "output"
logs_dir = "logs"
```

## API Reference

### Service Endpoints

**POST** `/v1/generate/speech`
```json
{
  "text": "Text to synthesize",
  "speaker_ref_path": "",
  "temperature": 0.75,
  "language": "en"
}
```

**GET** `/health`
```json
{
  "status": "healthy",
  "model_loaded": true,
  "service": "TTS"
}
```

### Client Usage

```bash
# Options
--text string           Text to convert to speech
--output string         Output WAV file path
--chunks string         JSON file with text chunks
--output-dir string     Directory for batch output
--verbose              Enable detailed logging
--health               Check service health
```

## Development

### Testing
```bash
make test          # Run all tests
make test-go       # Go tests only
make test-python   # Python tests only
```

### Code Quality
```bash
make lint          # Run golangci-lint
make fmt           # Format Go code
```

### Build
```bash
make build         # Build client binary
make install       # Install to ~/bin
make clean         # Clean artifacts
```

## Project Structure

```
├── cmd/
│   ├── go-client/     # Go CLI client
│   └── tts-service/   # Python FastAPI service
├── internal/
│   ├── config/        # Configuration management
│   └── tts/           # TTS implementation
├── models/            # TTS model files
├── input/             # Input text files
├── output/            # Generated audio files
└── scripts/           # Helper scripts
```

## Models

Supports OuteTTS quantized models:
- `Llama-OuteTTS-1.0-1B-Q8_0.gguf` (recommended)
- `Llama-OuteTTS-1.0-1B-FP16.gguf`

Models use llama.cpp backend for efficient inference.

## Requirements

- **Go** 1.25+
- **Python** 3.8+
- **Dependencies**: FastAPI, OuteTTS, uvicorn
- **Hardware**: GPU recommended for optimal performance