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
)

// ToContext adds variable to context.
func ToContext(ctx context.Context, key string, value interface{}) context.Context {
	return shared.VarToContext(ctx, key, value)
}

// FromContext returns variables from context.
func FromContext(ctx context.Context) map[string]interface{} {
	return shared.VarsFromContext(ctx)
}

// Fork instruments context with a storage of variables.
func Fork(ctx context.Context) (context.Context, *shared.Vars) {
	var v shared.Vars

	return v.Fork(ctx)
}

// Steps provides godog gherkin step definitions.
type Steps struct {
	JSONComparer assertjson.Comparer

	varPrefix  string
	generators map[string]func() (interface{}, error)
}

// ReplaceBytesFromFile replaces vars in file contents.
func (s *Steps) ReplaceBytesFromFile(ctx context.Context, filePath string) (context.Context, []byte, error) {
	body, err := ioutil.ReadFile(filePath) //nolint // File inclusion via variable during tests.
	if err != nil {
		return ctx, nil, err
	}

	return s.ReplaceBytes(ctx, body)
}

// ReplaceBytes replaces vars in bytes.
//
// This function can help to interpolate variables into predefined templates.
func (s *Steps) ReplaceBytes(ctx context.Context, body []byte) (context.Context, []byte, error) {
	var (
		err error
		vv  *shared.Vars
	)

	if json5.Valid(body) {
		if body, err = json5.Downgrade(body); err != nil {
			return ctx, nil, fmt.Errorf("failed to downgrade JSON5 to JSON: %w", err)
		}
	}

	if s != nil {
		ctx, vv = s.JSONComparer.Vars.Fork(ctx)
	} else {
		ctx, vv = Fork(ctx)
	}

	if vv != nil {
		varMap := vv.GetAll()
		varNames := make([]string, 0, len(varMap))
		varJV := make(map[string][]byte)

		for k, v := range vv.GetAll() {
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
	}

	return ctx, body, nil
}

// AssertBytes compares payloads and collects variables from JSON fields.
func (s *Steps) AssertBytes(ctx context.Context, expected, received []byte) (context.Context, error) {
	var jc assertjson.Comparer

	if s != nil {
		jc = s.JSONComparer
	} else {
		jc = assertjson.Comparer{
			IgnoreDiff: assertjson.IgnoreDiff,
		}
	}

	ctx, jc.Vars = jc.Vars.Fork(ctx)

	if (expected == nil || json5.Valid(expected)) && json5.Valid(received) {
		expected, err := json5.Downgrade(expected)
		if err != nil {
			return ctx, err
		}

		return ctx, jc.FailNotEqual(expected, received)
	}

	if !bytes.Equal(expected, received) {
		return ctx, fmt.Errorf("expected: %q, received: %q",
			string(expected), string(received))
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

	ctx, rv, err := s.ReplaceBytes(ctx, []byte(value))
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
	_, v := s.JSONComparer.Vars.Fork(ctx)

	_, rv, err := s.ReplaceBytes(ctx, []byte(value))
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
	ctx, v := s.JSONComparer.Vars.Fork(ctx)

	for _, row := range table.Rows {
		if len(row.Cells) != 2 {
			return ctx, fmt.Errorf("two columns expected in the table, %d received", len(row.Cells))
		}

		name := row.Cells[0].Value
		value := row.Cells[1].Value

		_, rv, err := s.ReplaceBytes(ctx, []byte(value))
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
	_, v := s.JSONComparer.Vars.Fork(ctx)

	for _, row := range table.Rows {
		if len(row.Cells) != 2 {
			return fmt.Errorf("two columns expected in the table, %d received", len(row.Cells))
		}

		name := row.Cells[0].Value
		value := row.Cells[1].Value

		_, rv, err := s.ReplaceBytes(ctx, []byte(value))
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
