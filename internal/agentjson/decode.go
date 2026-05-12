package agentjson

import (
	"encoding/json"
	"strings"
)

// DecodeFirst decodes the first JSON value from an agent response.
func DecodeFirst(raw string, out any) error {
	if err := decode(raw, out); err == nil {
		return nil
	} else {
		initialErr := err
		for idx, char := range raw {
			if idx == 0 || (char != '{' && char != '[') {
				continue
			}
			if err := decode(raw[idx:], out); err == nil {
				return nil
			}
		}
		return initialErr
	}
}

func decode(raw string, out any) error {
	decoder := json.NewDecoder(strings.NewReader(raw))
	return decoder.Decode(out)
}
