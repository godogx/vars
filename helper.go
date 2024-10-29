package vars

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/bool64/shared"
	"github.com/cucumber/godog"
	"github.com/swaggest/assertjson"
	"github.com/swaggest/assertjson/json5"
	"github.com/yalp/jsonpath"
)

// ToContext adds variable to context.
func ToContext(ctx context.Context, key string, value interface{}) context.Context {
	return shared.VarToContext(ctx, key, value)
}

// FromContext returns variables from context.
func FromContext(ctx context.Context) map[string]interface{} {
	return shared.VarsFromContext(ctx)
}

// Vars instruments context with a storage of variables.
func Vars(ctx context.Context) (context.Context, *shared.Vars) {
	var v shared.Vars

	return v.Fork(ctx)
}

// Replace replaces vars in bytes slice.
//
// This function can help to interpolate variables into predefined templates.
// It is generally used to prepare `expected` value.
func Replace(ctx context.Context, body []byte) (context.Context, []byte, error) {
	var v *Steps

	return v.Replace(ctx, body)
}

// ReplaceFile replaces vars in file contents.
//
// It works same as Replace using a file for input.
func ReplaceFile(ctx context.Context, filePath string) (context.Context, []byte, error) {
	var v *Steps

	return v.ReplaceFile(ctx, filePath)
}

// Assert compares payloads and collects variables from JSON fields.
func Assert(ctx context.Context, expected, received []byte, ignoreAddedJSONFields bool) (context.Context, error) {
	var v *Steps

	return v.Assert(ctx, expected, received, ignoreAddedJSONFields)
}

// AssertFile compares payloads and collects variables from JSON fields.
func AssertFile(ctx context.Context, filePath string, received []byte, ignoreAddedJSONFields bool) (context.Context, error) {
	var v *Steps

	return v.AssertFile(ctx, filePath, received, ignoreAddedJSONFields)
}

// AssertJSONPaths compares payload with a list of JSON path expectations.
func AssertJSONPaths(ctx context.Context, jsonPaths *godog.Table, received []byte, ignoreAddedJSONFields bool) (context.Context, error) {
	var v *Steps

	return v.AssertJSONPaths(ctx, jsonPaths, received, ignoreAddedJSONFields)
}

// Vars instruments context with a storage of variables.
func (s *Steps) Vars(ctx context.Context) (context.Context, *shared.Vars) {
	if s != nil {
		return s.JSONComparer.Vars.Fork(ctx)
	}

	return Vars(ctx)
}

func (s *Steps) jc(ctx context.Context) (context.Context, assertjson.Comparer) {
	var jc assertjson.Comparer

	if s != nil {
		jc = s.JSONComparer
	} else {
		jc = assertjson.Comparer{
			IgnoreDiff: assertjson.IgnoreDiff,
		}
	}

	ctx, jc.Vars = jc.Vars.Fork(ctx)

	return ctx, jc
}

// ReplaceFile replaces vars in file contents.
//
// It works same as Replace using a file for input.
func (s *Steps) ReplaceFile(ctx context.Context, filePath string) (context.Context, []byte, error) {
	body, err := os.ReadFile(filePath) //nolint // File inclusion via variable during tests.
	if err != nil {
		return ctx, nil, err
	}

	return s.Replace(ctx, body)
}

// Replace replaces vars in bytes slice.
//
// This function can help to interpolate variables into predefined templates.
// It is generally used to prepare `expected` value.
func (s *Steps) Replace(ctx context.Context, body []byte) (context.Context, []byte, error) {
	var err error

	if json5.Valid(body) {
		if body, err = json5.Downgrade(body); err != nil {
			return ctx, nil, fmt.Errorf("failed to downgrade JSON5 to JSON: %w", err)
		}
	}

	ctx, jc := s.jc(ctx)

	varMap := jc.Vars.GetAll()
	varNames := make([]string, 0, len(varMap))
	varJV := make(map[string][]byte)

	for k, v := range jc.Vars.GetAll() {
		varNames = append(varNames, k)

		jv, err := json.Marshal(v)
		if err != nil {
			return ctx, nil, fmt.Errorf("failed to marshal var %s (%v): %w", k, v, err)
		}

		varJV[k] = jv

		body = bytes.ReplaceAll(body, []byte(`"`+k+`"`), jv)
	}

	sort.Slice(varNames, func(i, j int) bool {
		return len(varNames[i]) > len(varNames[j])
	})

	for _, k := range varNames {
		jv := varJV[k]

		if jv[0] == '"' && jv[len(jv)-1] == '"' {
			jv = jv[1 : len(jv)-1]
		}

		body = bytes.ReplaceAll(body, []byte(k), jv)
	}

	return ctx, body, nil
}

// AssertFile compares payloads and collects variables from JSON fields.
func (s *Steps) AssertFile(ctx context.Context, filePath string, received []byte, ignoreAddedJSONFields bool) (context.Context, error) {
	body, err := os.ReadFile(filePath) //nolint // File inclusion via variable during tests.
	if err != nil {
		return ctx, err
	}

	return s.Assert(ctx, body, received, ignoreAddedJSONFields)
}

// PrepareContext makes sure context is instrumented with a valid comparer and does not need to be updated later.
func (s *Steps) PrepareContext(ctx context.Context) context.Context {
	ctx, _ = s.jc(ctx)

	return ctx
}

// Assert compares payloads and collects variables from JSON fields.
func (s *Steps) Assert(ctx context.Context, expected, received []byte, ignoreAddedJSONFields bool) (context.Context, error) {
	ctx, jc := s.jc(ctx)

	ctx, expected, err := s.Replace(ctx, expected)
	if err != nil {
		return ctx, err
	}

	if (expected == nil || json5.Valid(expected)) && json5.Valid(received) {
		expected, err := json5.Downgrade(expected)
		if err != nil {
			return ctx, err
		}

		if ignoreAddedJSONFields {
			return ctx, jc.FailMismatch(expected, received)
		}

		return ctx, jc.FailNotEqual(expected, received)
	}

	if !bytes.Equal(expected, received) {
		return ctx, fmt.Errorf("expected: %q, received: %q",
			string(expected), string(received))
	}

	return ctx, nil
}

// AssertJSONPaths compares payload with a list of JSON path expectations.
func (s *Steps) AssertJSONPaths(ctx context.Context, jsonPaths *godog.Table, received []byte, ignoreAddedJSONFields bool) (context.Context, error) {
	var rcv interface{}
	if err := json.Unmarshal(received, &rcv); err != nil {
		return ctx, fmt.Errorf("failed to unmarshal received value: %w", err)
	}

	ctx, jc := s.jc(ctx)

	for _, row := range jsonPaths.Rows {
		path := row.Cells[0].Value

		v, err := jsonpath.Read(rcv, path)
		if err != nil {
			return ctx, fmt.Errorf("failed to read jsonpath %s: %w", path, err)
		}

		actual, err := json.Marshal(v)
		if err != nil {
			return ctx, fmt.Errorf("failed to marshal actual value at jsonpath %s: %w", path, err)
		}

		expected := []byte(row.Cells[1].Value)

		_, expected, err = s.Replace(ctx, expected)
		if err != nil {
			return ctx, fmt.Errorf("failed to prepare expected value at jsonpath %s: %w", path, err)
		}

		if ignoreAddedJSONFields {
			err = jc.FailMismatch(expected, actual)
		} else {
			err = jc.FailNotEqual(expected, actual)
		}

		if err != nil {
			return ctx, fmt.Errorf("failed to assert jsonpath %s: %w", path, err)
		}
	}

	return ctx, nil
}
