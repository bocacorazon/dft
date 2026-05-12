package agentjson

import (
	"encoding/json"
	"strings"
)

// DecodeFirst decodes the first JSON value from an agent response.
func DecodeFirst(raw string, out any) error {
	decoder := json.NewDecoder(strings.NewReader(raw))
	return decoder.Decode(out)
}
