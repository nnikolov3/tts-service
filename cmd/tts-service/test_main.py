#!/usr/bin/env python3
"""
Test suite for the TTS FastAPI service following Test-Driven Development
principles.
Tests verify API contract compliance, error handling, and integration behavior.
"""

import os
import unittest
from unittest.mock import MagicMock, patch

from fastapi.testclient import TestClient

# Import the FastAPI app from main module
from main import TTSRequest, app, initialize_tts_service, on_startup


class TestTTSServiceAPI(unittest.TestCase):
    """Test suite for TTS service API endpoints following TDD principles."""

    def setUp(self):
        """Set up test client and mock dependencies for each test."""
        self.client = TestClient(app)

        # Mock the TTS interface to avoid requiring actual model files
        self.mock_tts_interface = MagicMock()
        self.mock_speaker = MagicMock()

        # Create mock audio output
        self.mock_audio_data = b"mock-wav-audio-data"
        self.mock_output = MagicMock()
        self.mock_output.save_to_io = MagicMock()

    def test_tts_request_validation_valid_input(self):
        """Test TTSRequest validation with valid input parameters."""
        valid_request = {
            "text": "Hello, world!",
            "speaker_ref_path": "/path/to/speaker.wav",
            "temperature": 0.75,
            "language": "en",
        }

        # This should not raise a validation error
        request = TTSRequest(**valid_request)
        self.assertEqual(request.text, "Hello, world!")
        self.assertEqual(request.temperature, 0.75)
        self.assertEqual(request.language, "en")

    def test_tts_request_validation_empty_text(self):
        """Test TTSRequest validation rejects empty text."""
        invalid_request = {"text": "", "temperature": 0.75, "language": "en"}

        with self.assertRaises(ValueError):
            TTSRequest(**invalid_request)

    def test_tts_request_validation_whitespace_only_text(self):
        """Test TTSRequest validation rejects whitespace-only text."""
        invalid_request = {
            "text": "   \n\t  ",
            "temperature": 0.75,
            "language": "en",
        }

        with self.assertRaises(ValueError):
            TTSRequest(**invalid_request)

    def test_tts_request_validation_temperature_out_of_range(self):
        """Test TTSRequest validation enforces temperature bounds."""
        # Test temperature too low
        with self.assertRaises(ValueError):
            TTSRequest(text="Hello", temperature=-0.1, language="en")

        # Test temperature too high
        with self.assertRaises(ValueError):
            TTSRequest(text="Hello", temperature=2.1, language="en")

    def test_tts_request_validation_text_too_long(self):
        """Test TTSRequest validation enforces text length limits."""
        long_text = "a" * 10001  # Exceeds max_length=10000

        with self.assertRaises(ValueError):
            TTSRequest(text=long_text, temperature=0.75, language="en")

    @patch("main.tts_interface")
    @patch("main.default_speaker")
    def test_generate_speech_success(self, mock_speaker, mock_interface):
        """Test successful speech generation via API endpoint."""
        # Configure mocks
        mock_interface.generate.return_value = self.mock_output
        mock_speaker.return_value = self.mock_speaker

        # Configure mock output to write audio data
        def mock_save_to_io(buffer):
            buffer.write(self.mock_audio_data)

        self.mock_output.save_to_io.side_effect = mock_save_to_io

        # Make request to API
        request_data = {
            "text": "Hello, this is a test.",
            "temperature": 0.8,
            "language": "en",
        }

        response = self.client.post("/v1/generate/speech", json=request_data)

        # Verify response
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.headers["content-type"], "audio/wav")
        self.assertEqual(response.content, self.mock_audio_data)

        # Verify TTS interface was called correctly
        mock_interface.generate.assert_called_once()

    def test_generate_speech_invalid_request_empty_text(self):
        """Test API rejects requests with empty text."""
        request_data = {"text": "", "temperature": 0.75, "language": "en"}

        response = self.client.post("/v1/generate/speech", json=request_data)

        self.assertEqual(response.status_code, 422)  # Validation error
        error_data = response.json()
        self.assertIn("detail", error_data)

    def test_generate_speech_invalid_request_missing_text(self):
        """Test API rejects requests missing required text field."""
        request_data = {"temperature": 0.75, "language": "en"}

        response = self.client.post("/v1/generate/speech", json=request_data)

        self.assertEqual(response.status_code, 422)  # Validation error

    def test_generate_speech_invalid_temperature(self):
        """Test API rejects requests with invalid temperature values."""
        # Test temperature too low
        request_data = {
            "text": "Hello, world!",
            "temperature": -0.5,
            "language": "en",
        }

        response = self.client.post("/v1/generate/speech", json=request_data)
        self.assertEqual(response.status_code, 422)

        # Test temperature too high
        request_data["temperature"] = 2.5
        response = self.client.post("/v1/generate/speech", json=request_data)
        self.assertEqual(response.status_code, 422)

    @patch("main.tts_interface", None)
    def test_generate_speech_service_not_initialized(self):
        """Test API returns error when TTS service is not initialized."""
        request_data = {
            "text": "Hello, world!",
            "temperature": 0.75,
            "language": "en",
        }

        response = self.client.post("/v1/generate/speech", json=request_data)

        self.assertEqual(response.status_code, 500)
        error_data = response.json()
        self.assertEqual(error_data["detail"], "TTS service not initialized")

    @patch("main.tts_interface")
    @patch("main.default_speaker")
    def test_generate_speech_generation_failure(self, mock_speaker, mock_interface):
        """Test API handles TTS generation failures gracefully."""
        # Configure mock to raise exception
        mock_interface.generate.side_effect = Exception("Model generation failed")
        mock_speaker.return_value = self.mock_speaker

        request_data = {
            "text": "Hello, world!",
            "temperature": 0.75,
            "language": "en",
        }

        response = self.client.post("/v1/generate/speech", json=request_data)

        self.assertEqual(response.status_code, 500)
        error_data = response.json()
        self.assertIn("Speech generation failed", error_data["detail"])

    def test_health_check_success(self):
        """Test health check endpoint returns success when service is healthy."""
        with (
            patch("main.tts_interface", self.mock_tts_interface),
            patch("main.default_speaker", self.mock_speaker),
        ):
            response = self.client.get("/health")

            self.assertEqual(response.status_code, 200)
            health_data = response.json()
            self.assertEqual(health_data["status"], "healthy")
            self.assertTrue(health_data["model_loaded"])

    def test_health_check_service_not_initialized(self):
        """Test health check reports when TTS interface is not loaded."""
        with patch("main.tts_interface", None):
            response = self.client.get("/health")

            self.assertEqual(response.status_code, 200)
            health_data = response.json()
            self.assertEqual(health_data["status"], "healthy")
            self.assertFalse(health_data["model_loaded"])


class TestTTSServiceInitialization(unittest.TestCase):
    """Test suite for TTS service initialization logic."""

    def test_initialize_tts_service_missing_model_file(self):
        """Test initialization fails gracefully when model file doesn't exist."""
        nonexistent_path = "/path/that/does/not/exist.gguf"

        with self.assertRaises(SystemExit):
            initialize_tts_service(nonexistent_path)

    @patch("main.outetts.Interface")
    @patch("main.Path.exists")
    def test_initialize_tts_service_success(self, mock_exists, mock_interface_class):
        """Test successful TTS service initialization."""
        mock_exists.return_value = True
        mock_interface = MagicMock()
        mock_interface_class.return_value = mock_interface
        mock_interface.load_default_speaker.return_value = MagicMock()

        test_model_path = "/tmp/test_model.gguf"

        # This should not raise an exception
        try:
            initialize_tts_service(test_model_path)
        except SystemExit:
            self.fail("initialize_tts_service raised SystemExit unexpectedly")

        # Verify interface was created with correct config
        mock_interface_class.assert_called_once()
        mock_interface.load_default_speaker.assert_called_once_with(
            "en-female-1-neutral"
        )

    @patch("main.outetts.Interface")
    @patch("main.Path.exists")
    def test_initialize_tts_service_interface_creation_failure(
        self, mock_exists, mock_interface_class
    ):
        """Test initialization handles OuteTTS interface creation failures."""
        mock_exists.return_value = True
        mock_interface_class.side_effect = Exception("CUDA not available")

        test_model_path = "/tmp/test_model.gguf"

        with self.assertRaises(SystemExit):
            initialize_tts_service(test_model_path)


class TestTTSServiceConfiguration(unittest.TestCase):
    """Test suite for TTS service configuration and environment handling."""

    def setUp(self):
        """Set up test environment."""
        # Store original environment
        self.original_env = os.environ.get("TTS_MODEL_PATH")

    def tearDown(self):
        """Restore original environment."""
        if self.original_env is not None:
            os.environ["TTS_MODEL_PATH"] = self.original_env
        elif "TTS_MODEL_PATH" in os.environ:
            del os.environ["TTS_MODEL_PATH"]

    def test_startup_event_missing_model_path_env(self):
        """Test startup event fails when TTS_MODEL_PATH is not set."""
        # Remove environment variable
        if "TTS_MODEL_PATH" in os.environ:
            del os.environ["TTS_MODEL_PATH"]

        # This should trigger a SystemExit in the startup event
        with self.assertRaises(SystemExit):
            # Import and trigger startup
            on_startup()

    @patch("main.initialize_tts_service")
    def test_startup_event_with_valid_model_path(self, mock_initialize):
        """Test startup event succeeds with valid TTS_MODEL_PATH."""
        test_model_path = "/tmp/test_model.gguf"
        os.environ["TTS_MODEL_PATH"] = test_model_path

        # Import and trigger startup
        on_startup()

        # Verify initialization was called with correct path
        mock_initialize.assert_called_once_with(test_model_path)


if __name__ == "__main__":
    # Run tests with verbose output
    unittest.main(verbosity=2)
