package vars

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"

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

// Steps provides godog gherkin step definitions.
type Steps struct {
	JSONComparer assertjson.Comparer

	varPrefix  string
	generators map[string]func() (interface{}, error)
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
func (s *Steps) ReplaceFile(ctx context.Context, filePath string) (context.Context, []byte, error) {
	body, err := ioutil.ReadFile(filePath) //nolint // File inclusion via variable during tests.
	if err != nil {
		return ctx, nil, err
	}

	return s.Replace(ctx, body)
}

// Replace replaces vars in bytes slice.
//
// This function can help to interpolate variables into predefined templates.
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
	body, err := ioutil.ReadFile(filePath) //nolint // File inclusion via variable during tests.
	if err != nil {
		return ctx, err
	}

	return s.Assert(ctx, body, received, ignoreAddedJSONFields)
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

		ctx, expected, err = s.Replace(ctx, expected)
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

// AddGenerator registers user-defined generator function, suitable for random identifiers.
func (s *Steps) AddGenerator(name string, f func() (interface{}, error)) {
	if s.generators == nil {
		s.generators = make(map[string]func() (interface{}, error))
	}

	s.generators[name] = f
}

// Register add steps to scenario context.
func (s *Steps) Register(sc *godog.ScenarioContext) {
	s.varPrefix = "$"
	if s.JSONComparer.Vars != nil && s.JSONComparer.Vars.VarPrefix != "" {
		s.varPrefix = s.JSONComparer.Vars.VarPrefix
	}

	// Given variable $foo is undefined
	sc.Step(`^variable \`+s.varPrefix+`([\w\d]+) is undefined$`, s.varIsUndefined)

	// When variable $foo is set to "abcdef"
	sc.Step(`^variable \`+s.varPrefix+`([\w\d]+) is set to (.+)$`, s.varIsSet)

	// When variable $foo is set to
	// """json5
	// {"foo":"bar"}
	// """
	sc.Step(`^variable \`+s.varPrefix+`([\w\d]+) is set to$`, s.varIsSet)

	// Then variable $foo equals to "abcdef"
	sc.Step(`^variable \`+s.varPrefix+`([\w\d]+) equals to (.+)$`, s.varEquals)

	//    When variables are set to values
	//      | $bar   | "abc"             |
	//      | $baz   | {"one":1,"two":2} |
	//      | $qux   | 123               |
	//      | $quux  | true              |
	//      | $corge | gen:alphanum-7    |
	sc.Step(`^variables are set to values$`, s.varsAreSet)

	//    Then variables are equal to values
	//      | $bar  | "abc"             |
	//      | $baz  | {"one":1,"two":2} |
	//      | $qux  | 123               |
	//      | $quux | true              |
	sc.Step(`^variables are equal to values$`, s.varsAreEqual)

	//    Then variable $bar matches JSON paths
	//      | $.foo          | "abcdef"   |
	//      | $.bar          | 123        |
	//      | $.baz          | true       |
	//      | $.prefixed_foo | "ooo::$foo" |
	sc.Step(`^variable \`+s.varPrefix+`([\w\d]+) matches JSON paths$`, s.varMatchesJSONPaths)
}

func (s *Steps) varIsUndefined(ctx context.Context, name string) error {
	_, v := s.JSONComparer.Vars.Fork(ctx)

	stored, found := v.Get(s.varPrefix + name)
	if found {
		val, err := json.Marshal(stored)
		if err != nil {
			return fmt.Errorf("variable %s is defined", name)
		}

		return fmt.Errorf("variable %s is defined with value %s", name, string(val))
	}

	return nil
}

func (s *Steps) varIsSet(ctx context.Context, name, value string) (context.Context, error) {
	ctx, v := s.JSONComparer.Vars.Fork(ctx)

	ctx, rv, err := s.Replace(ctx, []byte(value))
	if err != nil {
		return ctx, fmt.Errorf("replacing vars in %s: %w", value, err)
	}

	val, gen, err := s.gen(value)
	if err != nil {
		return ctx, err
	}

	if !gen {
		if err := json.Unmarshal(rv, &val); err != nil {
			return ctx, fmt.Errorf("decoding variable %s with value %s as JSON: %w", name, value, err)
		}
	}

	v.Set(s.varPrefix+name, val)

	return ctx, nil
}

func (s *Steps) gen(value string) (interface{}, bool, error) {
	if !strings.HasPrefix(value, "gen:") {
		return nil, false, nil
	}

	gen := value[4:]

	f, ok := s.generators[gen]
	if !ok {
		return nil, true, fmt.Errorf("missing generator %q", gen)
	}

	val, err := f()
	if err != nil {
		return nil, true, fmt.Errorf("generating value with %q: %w", gen, err)
	}

	return val, true, nil
}

func (s *Steps) varEquals(ctx context.Context, name, value string) error {
	_, v := s.Vars(ctx)

	_, rv, err := s.Replace(ctx, []byte(value))
	if err != nil {
		return fmt.Errorf("replacing vars in %s: %w", value, err)
	}

	stored, found := v.Get(s.varPrefix + name)
	if !found {
		return fmt.Errorf("could not find variable %s", name)
	}

	if err := assertjson.FailNotEqualMarshal(rv, stored); err != nil {
		return fmt.Errorf("variable %s assertion failed: %w", name, err)
	}

	return nil
}

func (s *Steps) varsAreSet(ctx context.Context, table *godog.Table) (context.Context, error) {
	ctx, v := s.Vars(ctx)

	for _, row := range table.Rows {
		if len(row.Cells) != 2 {
			return ctx, fmt.Errorf("two columns expected in the table, %d received", len(row.Cells))
		}

		name := row.Cells[0].Value
		value := row.Cells[1].Value

		_, rv, err := s.Replace(ctx, []byte(value))
		if err != nil {
			return ctx, fmt.Errorf("replacing vars in %s: %w", row.Cells[1].Value, err)
		}

		val, gen, err := s.gen(string(rv))
		if err != nil {
			return ctx, err
		}

		if !gen {
			if err := json.Unmarshal(rv, &val); err != nil {
				return ctx, fmt.Errorf("decoding variable %s with value %s as JSON: %w", name, value, err)
			}
		}

		v.Set(name, val)
	}

	return ctx, nil
}

func (s *Steps) varsAreEqual(ctx context.Context, table *godog.Table) error {
	_, v := s.Vars(ctx)

	for _, row := range table.Rows {
		if len(row.Cells) != 2 {
			return fmt.Errorf("two columns expected in the table, %d received", len(row.Cells))
		}

		name := row.Cells[0].Value
		value := row.Cells[1].Value

		_, rv, err := s.Replace(ctx, []byte(value))
		if err != nil {
			return fmt.Errorf("failed to replace vars in %s: %w", row.Cells[1].Value, err)
		}

		stored, found := v.Get(name)
		if !found {
			return fmt.Errorf("could not find variable %s", name)
		}

		if err := assertjson.FailNotEqualMarshal(rv, stored); err != nil {
			return fmt.Errorf("variable %s assertion failed: %w", name, err)
		}
	}

	return nil
}

func (s *Steps) varMatchesJSONPaths(ctx context.Context, name string, jsonPaths *godog.Table) (context.Context, error) {
	ctx, v := s.Vars(ctx)

	stored, found := v.Get(s.varPrefix + name)
	if !found {
		return ctx, fmt.Errorf("could not find variable %s", name)
	}

	j, err := json.Marshal(stored)
	if err != nil {
		return ctx, fmt.Errorf("failed to marshal variable %s: %w", name, err)
	}

	return s.AssertJSONPaths(ctx, jsonPaths, j, true)
}
