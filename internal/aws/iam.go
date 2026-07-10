package aws

import (
	"encoding/json"
	"fmt"
)

// GetCurrentIdentity returns the current AWS identity
func GetCurrentIdentity() (map[string]interface{}, error) {
	output, err := run.Run("aws", "sts", "get-caller-identity")
	if err != nil {
		return nil, fmt.Errorf("failed to get current identity: %w", err)
	}

	var identity map[string]interface{}
	if err := json.Unmarshal([]byte(output), &identity); err != nil {
		return nil, fmt.Errorf("failed to parse identity: %w", err)
	}

	return identity, nil
}
