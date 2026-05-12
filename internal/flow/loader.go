package flow

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadDefinition reads a minimal external flow definition from JSON.
func LoadDefinition(path string) (Definition, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Definition{}, fmt.Errorf("read flow definition: %w", err)
	}
	var definition Definition
	if err := json.Unmarshal(content, &definition); err != nil {
		return Definition{}, fmt.Errorf("parse flow definition: %w", err)
	}
	if definition.MaxSpecParallelism < 0 {
		return Definition{}, fmt.Errorf("max_spec_parallelism cannot be negative")
	}
	for _, step := range definition.Steps {
		if step.MaxIterations < 0 {
			return Definition{}, fmt.Errorf("step %q max_iterations cannot be negative", step.ID)
		}
	}
	return definition, nil
}
