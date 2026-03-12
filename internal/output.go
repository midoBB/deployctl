package internal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

type Field struct {
	Key   string
	Value any
}

func PrintOutput(w io.Writer, asJSON bool, fields []Field) error {
	if asJSON {
		payload := make(map[string]any, len(fields))
		for _, field := range fields {
			payload[field.Key] = normalizeJSONValue(field.Value)
		}

		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		return encoder.Encode(payload)
	}

	buffered := bufio.NewWriter(w)
	for _, field := range fields {
		if _, err := fmt.Fprintf(buffered, "%s=%s\n", field.Key, formatValue(field.Value)); err != nil {
			return err
		}
	}
	return buffered.Flush()
}

func normalizeJSONValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return ""
	case string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func formatValue(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(typed)
	}
}
