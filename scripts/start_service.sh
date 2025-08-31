#!/bin/bash
# Start the TTS HTTP service (for development/testing)

set -euo pipefail

# Global configuration variables
declare -r MODELS_DIR_GLOBAL="models"
declare -r VENV_DIR_GLOBAL="$HOME/Dev/.venv"
declare -r BIN_DIR_GLOBAL="$HOME/bin"
declare -r SERVICE_SCRIPT_GLOBAL="cmd/tts-service/main.py"
declare -r DEFAULT_HOST_GLOBAL="127.0.0.1"
declare -r DEFAULT_PORT_GLOBAL="8000"

# Runtime variables
declare MODEL_NAME_GLOBAL=""
declare HOST_GLOBAL=""
declare PORT_GLOBAL=""
declare MODEL_PATH_GLOBAL=""

function show_usage() {
    printf '%s\n' "Usage: $0 <model_name> [host] [port]"
    printf '%s\n' "Example: $0 Llama-OuteTTS-1.0-1B-Q8_0.gguf"
    printf '%s\n' "Models should be in the $MODELS_DIR_GLOBAL/ directory"
}

function validate_arguments() {
    if [[ -z "${1:-}" ]]; then
        show_usage
        exit 1
    fi

    MODEL_NAME_GLOBAL="$1"
    HOST_GLOBAL="${2:-$DEFAULT_HOST_GLOBAL}"
    PORT_GLOBAL="${3:-$DEFAULT_PORT_GLOBAL}"
    MODEL_PATH_GLOBAL="$MODELS_DIR_GLOBAL/$MODEL_NAME_GLOBAL"
}

function check_model_file() {
    local ls_result=""
    local ls_exit=""

    if [[ ! -f "$MODEL_PATH_GLOBAL" ]]; then
        printf '%s\n' "Error: Model file not found: $MODEL_PATH_GLOBAL" >&2
        printf '%s\n' "Available models:" >&2
        
        ls_result=$(ls -la "$MODELS_DIR_GLOBAL" 2>&1)
        ls_exit="$?"
        
        if [[ "$ls_exit" -eq 0 ]]; then
            printf '%s\n' "$ls_result" >&2
        else
            printf '%s\n' "No models directory found" >&2
        fi
        
        exit 1
    fi
}

function setup_python_environment() {
    local venv_check=""
    local venv_exit=""

    if [[ ! -d "$VENV_DIR_GLOBAL" ]]; then
        printf '%s\n' "Error: Virtual environment not found at $VENV_DIR_GLOBAL" >&2
        printf '%s\n' "Please ensure the venv exists or create it first" >&2
        exit 1
    fi

    # Activate existing venv
    # shellcheck source=/dev/null
    source "$VENV_DIR_GLOBAL/bin/activate"
    
    # Check if required packages are available
    venv_check=$(python -c "import outetts, fastapi, uvicorn; print('Dependencies available')" 2>&1)
    venv_exit="$?"
    
    if [[ "$venv_exit" -ne 0 ]]; then
        printf '%s\n' "Missing dependencies in venv: $venv_check" >&2
        printf '%s\n' "Please install: pip install fastapi uvicorn outetts" >&2
        exit 1
    fi
    
    printf '%s\n' "Python environment ready: $venv_check"
}

function start_tts_service() {
    printf '%s\n' "Starting TTS service..."
    printf '%s\n' "Model: $MODEL_PATH_GLOBAL"
    printf '%s\n' "Host: $HOST_GLOBAL"
    printf '%s\n' "Port: $PORT_GLOBAL"
    
    printf '%s\n' "Starting FastAPI TTS service..."
    python "$SERVICE_SCRIPT_GLOBAL" "$MODEL_PATH_GLOBAL" --host "$HOST_GLOBAL" --port "$PORT_GLOBAL"
}

function main() {
    validate_arguments "$@"
    check_model_file
    setup_python_environment
    start_tts_service
}

main "$@"