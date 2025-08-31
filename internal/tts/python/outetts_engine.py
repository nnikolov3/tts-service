#!/usr/bin/env python3
"""
Optimized OutTTS Engine for Go bindings.

This module provides optimized OutTTS functionality with improved performance,
better memory management, and updated dependencies to remove deprecation warnings.
"""

import argparse
import json
import logging
import signal
import sys
import threading
import warnings
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any, Dict, Optional, Union

# Audio playback imports
try:
    from pydub import AudioSegment
    from pydub.effects import normalize
    from pydub.playback import play

    PYDUB_AVAILABLE = True
except ImportError as e:
    PYDUB_AVAILABLE = False
    logger.error(f"Pydub not available for audio playback: {e}")

# Suppress deprecation warnings
warnings.filterwarnings("ignore", category=DeprecationWarning)
warnings.filterwarnings("ignore", category=FutureWarning)

# Constants
DEFAULT_GPU_MEMORY_GB = 5.5
MIN_GPU_MEMORY_GB = 1.0
DEFAULT_MAX_SEQ_LENGTH = 4096
DEFAULT_N_CTX = 4096
DEFAULT_ROPE_FREQ_BASE = 500000.0
DEFAULT_DAC_DECODING_CHUNK = 2048
DEFAULT_MAX_LENGTH = 8192
DEFAULT_DAC_DECODING_CHUNK_SMALL = 1536
DEFAULT_MAX_LENGTH_LARGE = 6144
DEFAULT_DAC_DECODING_CHUNK_TINY = 1024
DEFAULT_MAX_LENGTH_HUGE = 4096

# Configure logging
logging.basicConfig(
    level=logging.INFO, format="%(asctime)s - %(name)s - %(levelname)s - %(message)s"
)
logger = logging.getLogger(__name__)

# Global variables for process management
shutdown_event = threading.Event()
engine_instance = None

try:
    import outetts

    OUTETTS_AVAILABLE = True
    logger.info("OutTTS library is available")
except ImportError as e:
    OUTETTS_AVAILABLE = False
    logger.error(f"OutTTS library not available: {e}")

try:
    import torch

    TORCH_AVAILABLE = True
    logger.info("PyTorch libraries are available")
except ImportError as e:
    TORCH_AVAILABLE = False
    logger.error(f"PyTorch libraries not available: {e}")

try:
    PYDUB_AVAILABLE = True
    logger.info("Pydub audio library is available")
except ImportError as e:
    PYDUB_AVAILABLE = False
    logger.error(f"Pydub not available: {e}")
    logger.error(
        "Pydub is required for audio processing. Install with: pip install pydub"
    )


@dataclass
class OptimizedConfig:
    """Optimized configuration for OutTTS operations."""

    model_path: str
    backend: str = "LLAMACPP"
    device: str = "auto"
    max_workers: int = 8
    batch_size: int = 16
    timeout_seconds: int = 300
    gpu_memory_limit_gb: float = 5.5
    quality: str = "high"
    temperature: float = 0.4
    use_mirostat: bool = True


@dataclass
class SynthesisJob:
    """Represents a single synthesis job."""

    id: str
    text: str
    output_path: str
    quality: str = "high"
    temperature: float = 0.4
    speaker: str = "en-female-1-neutral"


@dataclass
class SynthesisResult:
    """Represents the result of a synthesis operation."""

    job_id: str
    success: bool
    error: Optional[str] = None
    duration: float = 0.0
    audio_path: str = ""
    audio_size: int = 0


class OuteTTSEngine:
    """Optimized OutTTS engine with improved performance and memory management."""

    def __init__(self, model_path: str, device: str = "auto"):
        """
        Initialize optimized OutTTS engine.

        Args:
            model_path: Path to the GGUF model file
            device: Device to use (ignored for llama.cpp backend)
        """
        if not OUTETTS_AVAILABLE:
            raise ImportError(
                "OutTTS library is not installed. Run: pip install outetts"
            )

        if not TORCH_AVAILABLE:
            raise ImportError("PyTorch libraries are not available")

        self.model_path = Path(model_path)
        if not self.model_path.exists():
            raise FileNotFoundError(f"Model file not found: {model_path}")

        self.interface = None
        self.default_speaker = None
        self.is_loaded = False
        self.mu = threading.Lock()

        # Memory management - more conservative for GPU usage
        self.gpu_memory_allocated = 0
        self.max_gpu_memory = (
            DEFAULT_GPU_MEMORY_GB * 1024 * 1024 * 1024
        )  # 2GB default - more conservative

        logger.info(
            f"Initializing optimized OutTTS engine with model: {self.model_path}"
        )
        self._load_model()

    def _load_model(self):
        """Load the OutTTS model with optimized settings."""
        try:
            with self.mu:
                # Create model configuration for local GGUF file
                config = outetts.ModelConfig(
                    model_path=str(self.model_path),
                    backend=outetts.Backend.LLAMACPP,
                    max_seq_length=DEFAULT_MAX_SEQ_LENGTH,  # Reduced for GPU memory
                    n_gpu_layers=8,  # Use fewer GPU layers to save memory
                    additional_model_config={
                        "n_ctx": DEFAULT_N_CTX,
                        "n_batch": 256,  # Reduced batch size
                        "n_threads": 4,
                        "n_gqa": 4,
                        "rope_freq_base": DEFAULT_ROPE_FREQ_BASE,
                    },
                )

                # Initialize interface
                self.interface = outetts.Interface(config=config)
                logger.info("OutTTS interface initialized successfully")

                # Load default speaker
                self.default_speaker = self.interface.load_default_speaker(
                    "en-female-1-neutral"
                )
                logger.info("Default speaker loaded")

                self.is_loaded = True
                logger.info("OutTTS model loaded successfully")

        except Exception as e:
            logger.error(f"Failed to load OutTTS model: {e}")
            raise

    def _get_quality_settings(self, quality: str) -> Dict[str, Any]:
        """Get optimized settings for different quality levels."""
        if quality == "fast":
            return {
                "generation_type": outetts.GenerationType.REGULAR,
                "max_batch_size": 16,
                "dac_decoding_chunk": DEFAULT_DAC_DECODING_CHUNK,
                "max_length": DEFAULT_MAX_LENGTH,
                "temperature": 0.6,
                "repetition_penalty": 1.05,
            }
        elif quality == "balanced":
            return {
                "generation_type": outetts.GenerationType.CHUNKED,
                "max_batch_size": 12,
                "dac_decoding_chunk": DEFAULT_DAC_DECODING_CHUNK_SMALL,
                "max_length": DEFAULT_MAX_LENGTH_LARGE,
                "temperature": 0.5,
                "repetition_penalty": 1.08,
            }
        else:  # high quality
            return {
                "generation_type": outetts.GenerationType.CHUNKED,
                "max_batch_size": 8,
                "dac_decoding_chunk": DEFAULT_DAC_DECODING_CHUNK_TINY,
                "max_length": DEFAULT_MAX_LENGTH_HUGE,
                "temperature": 0.4,
                "repetition_penalty": 1.1,
            }

    def _check_memory_usage(self) -> bool:
        """Check if we have enough GPU memory for processing."""
        if torch.cuda.is_available():
            try:
                allocated = torch.cuda.memory_allocated()
                reserved = torch.cuda.memory_reserved()
                total = torch.cuda.get_device_properties(0).total_memory

                available = total - reserved
                logger.info(
                    f"GPU memory: {allocated / 1024**3:.2f}GB allocated, "
                    f"{reserved / 1024**3:.2f}GB reserved, {available / 1024**3:.2f}GB available"
                )

                # More permissive memory check
                if available < MIN_GPU_MEMORY_GB * 1024 * 1024 * 1024:  # 1GB minimum
                    logger.warning(
                        f"Very low GPU memory: {available / 1024**3:.2f}GB available"
                    )
                    return False
                return True
            except Exception as e:
                logger.warning(f"Failed to check GPU memory: {e}")
                return True
        return True

    def _cleanup_memory(self):
        """Clean up GPU memory."""
        if torch.cuda.is_available():
            try:
                torch.cuda.empty_cache()
                torch.cuda.synchronize()
                logger.debug("GPU memory cleaned up")
            except Exception as e:
                logger.warning(f"Failed to cleanup GPU memory: {e}")

    def synthesize(
        self,
        text: str,
        output_path: str,
        speaker_id: Optional[str] = None,
        language: str = "en",
        quality: str = "high",
        temperature: float = 0.4,
        use_mirostat: bool = True,
    ) -> bool:
        """
        Synthesize speech from text using optimized settings.

        Args:
            text: Input text to synthesize
            output_path: Path to save the audio file
            speaker_id: Speaker ID (ignored, uses default speaker)
            language: Language code (ignored, model determines from text)

        Returns:
            bool: True if successful, False otherwise
        """
        if not self.is_loaded:
            logger.error("OutTTS model not loaded")
            return False

        try:
            # Check memory availability
            if not self._check_memory_usage():
                logger.error("Insufficient GPU memory")
                return False

            # Clean and validate text
            text = text.strip()
            if not text:
                logger.error("Empty text provided")
                return False

            logger.info(f"Synthesizing text: {text[:50]}...")

            # Get quality settings
            quality_settings = self._get_quality_settings(quality)

            # Create sampler config with optimized settings
            sampler_config = outetts.SamplerConfig(
                temperature=temperature,
                repetition_penalty=quality_settings["repetition_penalty"],
                repetition_range=64,  # Essential - must be 64
                top_k=40,
                top_p=0.9,
                min_p=0.05,
                mirostat=use_mirostat,
                mirostat_tau=5.0,
                mirostat_eta=0.1,
            )

            # Generate speech using OutTTS with optimized settings
            output = self.interface.generate(
                config=outetts.GenerationConfig(
                    text=text,
                    speaker=self.default_speaker,
                    generation_type=quality_settings["generation_type"],
                    max_batch_size=quality_settings["max_batch_size"],
                    dac_decoding_chunk=quality_settings["dac_decoding_chunk"],
                    max_length=quality_settings["max_length"],
                    sampler_config=sampler_config,
                )
            )

            # Save the generated audio
            output.save(output_path)
            logger.info(f"Audio saved to {output_path}")

            # Cleanup memory
            self._cleanup_memory()

            return True

        except Exception as e:
            logger.error(f"OutTTS synthesis failed: {e}")
            # Cleanup memory on error
            self._cleanup_memory()
            return False

    def play_audio(
        self, file_path: str, quality_settings: Optional[Dict] = None
    ) -> Dict:
        """
        Play audio with high-quality settings using pydub.

        Args:
            file_path: Path to the audio file
            quality_settings: Dictionary containing quality parameters

        Returns:
            Dictionary with playback result
        """
        if not PYDUB_AVAILABLE:
            return {"success": False, "error": "Pydub not available for audio playback"}

        try:
            # Load audio file
            logger.info(f"Loading audio file for playback: {file_path}")
            audio = AudioSegment.from_file(file_path)

            # Apply quality settings if provided
            if quality_settings:
                if quality_settings.get("sample_rate"):
                    audio = audio.set_frame_rate(quality_settings["sample_rate"])
                if quality_settings.get("channels"):
                    audio = audio.set_channels(quality_settings["channels"])
                if quality_settings.get("bit_depth") == 32:
                    audio = audio.set_sample_width(4)

                # Apply effects
                if quality_settings.get("volume", 1.0) != 1.0:
                    volume_db = 20 * (quality_settings["volume"] - 1.0)
                    audio = audio + volume_db

                if quality_settings.get("fade_in"):
                    fade_ms = int(quality_settings["fade_in"] * 1000)
                    audio = audio.fade_in(fade_ms)

                if quality_settings.get("fade_out"):
                    fade_ms = int(quality_settings["fade_out"] * 1000)
                    audio = audio.fade_out(fade_ms)

                if quality_settings.get("normalize", False):
                    audio = normalize(audio)

                # Apply filters if specified
                if quality_settings.get("high_pass"):
                    audio = audio.high_pass_filter(quality_settings["high_pass"])
                if quality_settings.get("low_pass"):
                    audio = audio.low_pass_filter(quality_settings["low_pass"])

            # Play the audio
            logger.info(
                f"Playing audio: {audio.duration_seconds:.2f}s, {audio.frame_rate}Hz, {audio.channels} channels"
            )
            play(audio)

            return {
                "success": True,
                "duration": audio.duration_seconds,
                "sample_rate": audio.frame_rate,
                "channels": audio.channels,
                "bit_depth": audio.sample_width * 8,
            }

        except Exception as e:
            logger.error(f"Error during audio playback: {e}")
            return {"success": False, "error": str(e)}

    def get_audio_info(self, file_path: str) -> Dict:
        """
        Get information about an audio file.

        Args:
            file_path: Path to the audio file

        Returns:
            Dictionary with audio file information
        """
        if not PYDUB_AVAILABLE:
            return {"success": False, "error": "Pydub not available for audio info"}

        try:
            audio = AudioSegment.from_file(file_path)

            # Get file size
            file_size = (
                Path(file_path).stat().st_size if Path(file_path).exists() else 0
            )

            # Determine format from file extension
            format_ext = Path(file_path).suffix.lower().lstrip(".")

            return {
                "success": True,
                "duration": audio.duration_seconds,
                "file_size": file_size,
                "format": format_ext,
                "sample_rate": audio.frame_rate,
                "channels": audio.channels,
                "bit_depth": audio.sample_width * 8,
            }

        except Exception as e:
            logger.error(f"Error getting audio info: {e}")
            return {"success": False, "error": str(e)}

    def get_model_info(self) -> Dict[str, Union[str, int, bool]]:
        """Get information about the loaded OutTTS model."""
        memory_info = {
            "model_loaded": self.is_loaded,
            "gpu_available": torch.cuda.is_available(),
        }

        if torch.cuda.is_available():
            try:
                memory_info.update(
                    {
                        "gpu_allocated_gb": torch.cuda.memory_allocated() / 1024**3,
                        "gpu_reserved_gb": torch.cuda.memory_reserved() / 1024**3,
                        "gpu_total_gb": torch.cuda.get_device_properties(0).total_memory
                        / 1024**3,
                        "gpu_device_count": torch.cuda.device_count(),
                    }
                )
            except Exception as e:
                memory_info["gpu_error"] = str(e)

        return {
            "model_path": str(self.model_path),
            "backend": "LLAMACPP",
            "is_loaded": self.is_loaded,
            "outetts_available": OUTETTS_AVAILABLE,
            "torch_available": TORCH_AVAILABLE,
            "model_type": "OutTTS GGUF",
            "default_speaker": "en-female-1-neutral" if self.default_speaker else None,
            "memory_usage": memory_info,
        }


def signal_handler(signum, frame):
    """Handle shutdown signals."""
    logger.info(f"Received signal {signum}, shutting down...")
    shutdown_event.set()
    if engine_instance:
        engine_instance.cleanup()
    sys.exit(0)


def main():
    """Command-line interface for testing."""
    parser = argparse.ArgumentParser(description="Optimized OutTTS Engine CLI")
    parser.add_argument("model_path", help="Path to GGUF model file")
    parser.add_argument("text", nargs="?", help="Text to synthesize")
    parser.add_argument("output_path", nargs="?", help="Output WAV file path")
    parser.add_argument("--speaker", help="Speaker ID (ignored)")
    parser.add_argument("--language", help="Language code (ignored)")
    parser.add_argument("--info", action="store_true", help="Show model info only")
    parser.add_argument(
        "--quality",
        choices=["fast", "balanced", "high"],
        default="high",
        help="Quality preset: fast, balanced, or high (default)",
    )
    parser.add_argument(
        "--temperature",
        type=float,
        default=0.4,
        help="Temperature for generation (default: 0.4)",
    )
    parser.add_argument(
        "--mirostat",
        action="store_true",
        default=True,
        help="Enable mirostat sampling for better quality",
    )
    parser.add_argument(
        "--persistent", action="store_true", help="Run in persistent mode"
    )

    args = parser.parse_args()

    # Validate required arguments for non-info/persistent modes
    if not args.info and not args.persistent:
        if not args.text or not args.output_path:
            parser.error(
                "text and output_path are required when not using --info or --persistent"
            )

    # Set up signal handlers
    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    try:
        engine = OuteTTSEngine(args.model_path)

        if args.info:
            print(json.dumps(engine.get_model_info(), indent=2))
            return

        if args.persistent:
            # Run in persistent mode for Go integration
            global engine_instance
            engine_instance = engine
            print("READY", flush=True)

            # Read from stdin for commands
            for line in sys.stdin:
                if shutdown_event.is_set():
                    break

                try:
                    command = json.loads(line.strip())
                    command_type = command.get("type")

                    if command_type == "single":
                        job_data = command["job"]
                        job = SynthesisJob(**job_data)
                        success = engine.synthesize(
                            job.text,
                            job.output_path,
                            quality=job.quality,
                            temperature=job.temperature,
                            use_mirostat=True,
                        )
                        result = SynthesisResult(
                            job_id=job.id, success=success, audio_path=job.output_path
                        )
                        print(json.dumps(asdict(result)), flush=True)

                    elif command_type == "memory_usage":
                        usage = engine.get_model_info()["memory_usage"]
                        print(json.dumps(usage), flush=True)

                    elif command_type == "play_audio":
                        file_path = command.get("file_path")
                        quality_settings = command.get("quality_settings", {})
                        if file_path:
                            result = engine.play_audio(file_path, quality_settings)
                            print(json.dumps(result), flush=True)
                        else:
                            print(
                                json.dumps(
                                    {
                                        "error": "Missing file_path for play_audio command"
                                    }
                                ),
                                flush=True,
                            )

                    elif command_type == "audio_info":
                        file_path = command.get("file_path")
                        if file_path:
                            result = engine.get_audio_info(file_path)
                            print(json.dumps(result), flush=True)
                        else:
                            print(
                                json.dumps(
                                    {
                                        "error": "Missing file_path for audio_info command"
                                    }
                                ),
                                flush=True,
                            )

                    elif command_type == "cleanup":
                        engine._cleanup_memory()
                        print(json.dumps({"status": "cleaned"}), flush=True)
                        break

                    else:
                        print(
                            json.dumps(
                                {"error": f"Unknown command type: {command_type}"}
                            ),
                            flush=True,
                        )

                except json.JSONDecodeError as e:
                    print(json.dumps({"error": f"Invalid JSON: {e}"}), flush=True)
                except Exception as e:
                    print(json.dumps({"error": f"Command failed: {e}"}), flush=True)

        else:
            # Regular CLI mode
            success = engine.synthesize(
                args.text,
                args.output_path,
                quality=args.quality,
                temperature=args.temperature,
                use_mirostat=args.mirostat,
            )

            if success:
                print(f"Successfully synthesized: {args.output_path}")
                print("Model info:", json.dumps(engine.get_model_info(), indent=2))
            else:
                print("Synthesis failed")
                sys.exit(1)

    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
