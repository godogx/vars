package vars

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Infer parses a value from a string into a suitable type.
// If s only contains digits, int64 is returned.
// If s is parsed as float, float64 is returned.
// If s has unquoted values of true or false, bool is returned.
// If s is equal to unquoted null, nil is returned.
// If s starts with double quote ", string decoded from JSON is returned (mind JSON escaping rules).
// If s starts with [ or {, any decoded from JSON is returned.
// Otherwise, s is returned as is.
func Infer(s string) interface{} {
	switch s {
	case "":
		return ""
	case "true":
		return true
	case "false":
		return false
	case "null":
		return nil
	}

	if s[0] == '"' {
		var v string
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			return fmt.Errorf("infer string value %s: %w", s, err)
		}

		return v
	}

	if s[0] == '[' || s[0] == '{' {
		var v interface{}
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			return fmt.Errorf("infer JSON value %s: %w", s, err)
		}

		return v
	}

	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}

	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}

	return s
}
