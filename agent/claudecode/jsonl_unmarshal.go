package claudecode

import "encoding/json"

// UnmarshalJSON handles the polymorphic content field in Claude's JSONL.
// User prompts have content as a plain string, while assistant responses
// and tool results have content as an array of ContentBlock objects.
func (cf *ContentField) UnmarshalJSON(data []byte) error {
	// Try string first (user prompts).
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		cf.Text = text
		return nil
	}

	// Try array of content blocks (assistant responses, tool results).
	var blocks []ContentBlock
	if err := json.Unmarshal(data, &blocks); err == nil {
		cf.Blocks = blocks
		return nil
	}

	// Unknown format — ignore silently for forward compatibility.
	return nil
}
