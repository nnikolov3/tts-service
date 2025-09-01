#!/usr/bin/env python3
"""
Standalone FastAPI TTS Service using OuteTTS with llama.cpp backend.

This service provides a decoupled, HTTP-based interface to OuteTTS
functionality,
following the microservice architecture principles defined in the design
blueprint.
The service loads the TTS model once at startup for optimal performance and
exposes a simple, explicit REST API for text-to-speech generation.

Key design principles implemented:
- Abstraction and modularity: Separates TTS logic from client applications
- Make the common case fast: Model loaded once at startup
- Explicit interfaces: Clear JSON request/response contracts
- Fail fast: Input validation at API boundaries
- Self-documenting: Clear types and comprehensive error messages
"""

import argparse
import io
import logging
import os
import sys
import tempfile
from pathlib import Path
from typing import Any, Optional

import outetts
import uvicorn
from fastapi import FastAPI, HTTPException
from fastapi.concurrency import run_in_threadpool
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field, field_validator

# Configure structured logging for service monitoring
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)

# Global TTS interface and speaker instances.
# These are loaded once at startup to adhere to the "Make the common case
# fast" principle,
# eliminating the significant latency of model loading for every individual
# request.
tts_interface: Optional[outetts.Interface] = None
default_speaker = None

# --- NEW: Secure mapping for custom speakers ---
# Maps a safe, client-provided ID to a secure, server-side file path.
# This prevents path traversal vulnerabilities.
# In a real application, this might be loaded from a config file.
SPEAKER_MAPPING = {
    "custom-voice-1": "/path/to/secure/speakers/voice1.wav",
    "custom-voice-2": "/path/to/secure/speakers/voice2.wav",
}


# API Request and Response Models
# These models ensure the API contract is explicit and self-documenting,
# providing compile-time validation and runtime input sanitization.


class TTSRequest(BaseModel):
    """Request model for TTS generation with comprehensive input validation.

    This model enforces the API contract by validating all input parameters
    at the service boundary, ensuring data integrity and preventing invalid
    requests from reaching the TTS processing logic.
    """

    # Text content to convert to speech - core required parameter
    text: str = Field(
        ...,
        min_length=1,
        max_length=10000,
        description="Text to convert to speech (1-10000 characters)",
    )

    # --- MODIFIED: Changed from path to secure ID ---
    # Optional server-side ID for voice cloning capabilities
    speaker_id: Optional[str] = Field(
        None,
        description="A predefined server-side speaker ID for voice cloning "
        "(e.g., 'custom-voice-1')",
    )

    # Generation randomness control parameter
    temperature: float = Field(
        0.75,
        ge=0.0,
        le=2.0,
        description="Generation temperature: 0.0 (deterministic) to 2.0 "
        "(highly random)",
    )

    # Target language for speech generation
    language: str = Field(
        "en", description="ISO language code (e.g., 'en', 'es', 'fr')"
    )

    @field_validator("text")
    @classmethod
    def validate_text_content(cls, v: str) -> str:
        """Validate text content is not empty or whitespace-only."""
        if not v or not v.strip():
            raise ValueError("Text cannot be empty or whitespace-only")
        return v.strip()


class ErrorResponse(BaseModel):
    """Structured error response model for providing actionable diagnostics.

    This model ensures error responses follow a consistent format that clients
    can parse programmatically while providing both human-readable descriptions
    and machine-readable error codes for automated error handling.
    """

    # Human-readable error description
    detail: str

    # Machine-readable error classification for automated handling
    error_code: str


# Service Initialization Functions
# These functions handle the critical startup phase where the TTS model is
# loaded into memory and configured for optimal performance.


def initialize_tts_service(model_path: str) -> tuple[outetts.Interface, Any]:
    """Initialize the OuteTTS service and return the interface and speaker.

    This function performs the heavy lifting of model loading at service startup,
    implementing the "Make the common case fast" principle by avoiding model

    Args:
        model_path: Absolute path to the OuteTTS model file (.gguf format)

    Returns:
        A tuple containing the initialized OuteTTS interface and default speaker.

    Raises:
        SystemExit: If model loading fails for any reason.
    """
    try:
        logger.info(f"Loading OuteTTS model from: {model_path}")

        if not Path(model_path).exists():
            raise FileNotFoundError(f"Model file not found at {model_path}")

        model_config = outetts.ModelConfig(
            model_path=model_path,
            backend=outetts.Backend.LLAMACPP,
            n_gpu_layers=99,
            max_seq_length=8192,
            additional_model_config={
                "n_ctx": 8192,
                "n_batch": 512,
            },
        )

        interface = outetts.Interface(config=model_config)
        logger.info("OuteTTS interface initialized with llama.cpp backend")

        speaker = interface.load_default_speaker("en-female-1-neutral")
        logger.info("Default speaker loaded and ready for generation")

        return interface, speaker

    except Exception as initialization_error:
        logger.error(f"FATAL: Failed to initialize TTS service: {initialization_error}")
        sys.exit(1)


# FastAPI Application Configuration
# The application is configured with explicit metadata for API documentation
# and service identification, following the principle of explicit interfaces.

app = FastAPI(
    title="TTS Microservice",
    description="Decoupled Text-to-Speech service using OuteTTS with llama.cpp backend",
    version="1.0.0",
    docs_url="/docs",  # Explicit API documentation endpoint
    redoc_url="/redoc",  # Alternative documentation interface
)


@app.on_event("startup")
def on_startup() -> None:
    """Application startup event handler for TTS model initialization.

    This function is called once when the FastAPI application starts,
    ensuring the TTS model is loaded and ready before accepting requests.
    The startup process validates environment configuration and performs
    the expensive model loading operation upfront.

    Environment Variables:
        TTS_MODEL_PATH: Required path to the OuteTTS model file

    Raises:
        SystemExit: If TTS_MODEL_PATH is not set or model loading fails
    """
    global tts_interface, default_speaker
    model_path = os.getenv("TTS_MODEL_PATH")
    if not model_path:
        logger.error("FATAL: TTS_MODEL_PATH environment variable not set")
        logger.error("Set TTS_MODEL_PATH to the path of your .gguf model file")
        sys.exit(1)

    logger.info("Starting TTS service initialization")
    tts_interface, default_speaker = initialize_tts_service(model_path)
    logger.info("TTS service startup completed successfully")


@app.post("/v1/generate/speech", response_class=StreamingResponse)
async def generate_speech(request: TTSRequest) -> StreamingResponse:
    """Generate speech audio from text input via the primary TTS endpoint.

    This endpoint represents the core functionality of the TTS microservice,
    accepting validated text input and returning WAV audio data. The
    implementation follows the explicit API contract defined in the service
    blueprint.

    Args:
        request: Validated TTSRequest containing text and generation parameters

    Returns:
        StreamingResponse: WAV audio data with appropriate content-type headers

    Raises:
        HTTPException:
            - 500 if TTS service is not initialized
            - 500 if speech generation fails for any reason
            - 400 if a custom speaker_id is invalid
            - 422 if request validation fails (handled by FastAPI)

    API Contract:
        - POST /v1/generate/speech
        - Content-Type: application/json (request)
        - Accept: audio/wav (response)
        - Response Content-Type: audio/wav
    """
    # Verify service initialization state
    if not tts_interface:
        logger.error("Speech generation attempted before service initialization")
        raise HTTPException(status_code=500, detail="TTS service not initialized")

    try:
        speaker = default_speaker

        # --- NEW: Logic to handle custom speaker ID ---
        if request.speaker_id:
            speaker_path = SPEAKER_MAPPING.get(request.speaker_id)
            if not speaker_path or not Path(speaker_path).exists():
                logger.warning(f"Invalid or unknown speaker_id: {request.speaker_id}")
                raise HTTPException(
                    status_code=400,
                    detail=f"Invalid speaker_id: '{request.speaker_id}' not found.",
                )
            try:
                logger.info(
                    f"Loading custom speaker '{request.speaker_id}' from "
                    f"path: {speaker_path}"
                )
                speaker = await run_in_threadpool(
                    tts_interface.load_speaker, speaker_path
                )
            except Exception as e:
                logger.error(
                    f"Failed to load custom speaker '{request.speaker_id}': {e}"
                )
                raise HTTPException(
                    status_code=500, detail="Error loading custom speaker."
                )

        if not speaker:
            logger.error("Speaker not available (default or custom)")
            raise HTTPException(
                status_code=500,
                detail="Speaker could not be loaded for generation",
            )

        # Configure generation parameters with explicit values
        sampler_config = outetts.SamplerConfig(temperature=request.temperature)

        generation_config = outetts.GenerationConfig(
            text=request.text,
            speaker=speaker,
            sampler_config=sampler_config,
            generation_type=outetts.GenerationType.CHUNKED,
        )

        logger.info(
            f"Generating speech for text length: {len(request.text)} characters"
        )

        # --- MODIFIED: Run blocking I/O in a thread pool ---
        # Generate audio using OuteTTS interface without blocking the
        # event loop
        output = await run_in_threadpool(
            tts_interface.generate, config=generation_config
        )
        with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as temp_file:
            output.save(temp_file.name)

        # Prepare audio data for streaming response
        audio_bytes = io.BytesIO()
        with open(temp_file.name, "rb") as f:
            audio_bytes.write(f.read())
        audio_bytes.seek(0)
        os.unlink(temp_file.name)

        audio_bytes.seek(0)

        logger.info(
            f"Speech generation completed, audio size: "
            f"{len(audio_bytes.getvalue())} bytes"
        )

        # Return streaming response with explicit content type
        return StreamingResponse(
            audio_bytes,
            media_type="audio/wav",
            headers={"Content-Disposition": "attachment; filename=speech.wav"},
        )

    except HTTPException:
        # Re-raise HTTPExceptions to avoid them being caught by the
        # generic Exception handler
        raise
    except Exception as generation_error:
        logger.error(f"Speech generation failed: {generation_error}")
        raise HTTPException(
            status_code=500,
            detail=f"Speech generation failed: {generation_error!s}",
        )


@app.get("/health")
async def health_check() -> dict[str, str | bool]:
    """Service health monitoring endpoint for operational readiness checks.

    This endpoint provides a lightweight mechanism for external systems
    to verify that the TTS service is operational and ready to process
    requests.
    It checks both service availability and model loading status.

    Returns:
        dict: Health status information containing:
            - status: Always "healthy" if endpoint responds
            - model_loaded: Boolean indicating if TTS model is ready

    API Contract:
        - GET /health
        - Response: 200 OK with JSON health status
        - No authentication required for monitoring purposes
    """
    model_ready = tts_interface is not None and default_speaker is not None

    return {
        "status": "healthy",
        "model_loaded": model_ready,
        "service": "tts-microservice",
        "version": "1.0.0",
    }


def main() -> None:
    """Main entry point for the TTS microservice application.

    This function handles command-line argument parsing, environment setup,
    and Uvicorn server configuration. It provides a clean interface for
    starting the service in both development and production environments.

    Command Line Arguments:
        model_path: Required path to the OuteTTS model file (.gguf format)
        --host: Server bind address (default: 127.0.0.1)
        --port: Server port number (default: 8000)

    Environment Variables Set:
        TTS_MODEL_PATH: Set from command line argument for startup handler
    """
    # Configure command-line argument parsing with comprehensive help
    parser = argparse.ArgumentParser(
        description="Standalone TTS Microservice using OuteTTS and llama.cpp",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s /path/to/model.gguf
  %(prog)s /path/to/model.gguf --host 0.0.0.0 --port 8080
        """,
    )

    parser.add_argument(
        "model_path", help="Path to the OuteTTS model file (.gguf format)"
    )
    parser.add_argument(
        "--host",
        default="127.0.0.1",
        help="Host address to bind server (default: %(default)s)",
    )
    parser.add_argument(
        "--port",
        type=int,
        default=8000,
        help="Port number to bind server (default: %(default)d)",
    )

    args = parser.parse_args()

    # Set environment variable for startup handler
    os.environ["TTS_MODEL_PATH"] = args.model_path

    logger.info(f"Starting TTS microservice on {args.host}:{args.port}")
    logger.info(f"Model path: {args.model_path}")

    # Start Uvicorn server with explicit configuration
    uvicorn.run(app, host=args.host, port=args.port, log_level="info", access_log=True)


if __name__ == "__main__":
    main()
