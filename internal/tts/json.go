package tts

import (
	"encoding/json"
	"fmt"
)

// parseJSON parses JSON data into the target interface.
func parseJSON(data []byte, target any) error {
	err := json.Unmarshal(data, target)
	if err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}
