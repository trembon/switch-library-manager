package db

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// FlexibleStrings decodes optional list metadata from the remote title
// catalog. It accepts a JSON array of strings, a single string, or null.
// Values of any other type, and non-string array entries, are ignored so one
// malformed title cannot reject the whole catalog.
type FlexibleStrings []string

func (values *FlexibleStrings) UnmarshalJSON(data []byte) error {
	*values = nil
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil
	}
	switch data[0] {
	case '[':
		var items []json.RawMessage
		if err := json.Unmarshal(data, &items); err != nil {
			return nil
		}
		result := make(FlexibleStrings, 0, len(items))
		for _, item := range items {
			var value string
			if json.Unmarshal(item, &value) == nil {
				if value = strings.TrimSpace(value); value != "" {
					result = append(result, value)
				}
			}
		}
		if len(result) > 0 {
			*values = result
		}
	case '"':
		var value string
		if json.Unmarshal(data, &value) == nil {
			if value = strings.TrimSpace(value); value != "" {
				*values = FlexibleStrings{value}
			}
		}
	}
	return nil
}

// FlexibleInt decodes optional numeric metadata from the remote title
// catalog. It accepts a JSON integer or a string holding one; anything else,
// including null, decodes as zero.
type FlexibleInt int

func (value *FlexibleInt) UnmarshalJSON(data []byte) error {
	*value = 0
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil
	}
	text := string(data)
	if data[0] == '"' {
		if json.Unmarshal(data, &text) != nil {
			return nil
		}
		text = strings.TrimSpace(text)
	}
	if parsed, err := strconv.Atoi(text); err == nil {
		*value = FlexibleInt(parsed)
	}
	return nil
}
