package vars

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/bool64/shared"
	"github.com/cucumber/godog"
	"github.com/swaggest/assertjson"
)

// ToContext adds variable to context.
func ToContext(ctx context.Context, key string, value interface{}) context.Context {
	return shared.VarToContext(ctx, key, value)
}

// FromContext returns variables from context.
func FromContext(ctx context.Context) map[string]interface{} {
	return shared.VarsFromContext(ctx)
}

// Steps provides godog gherkin step definitions.
type Steps struct {
	JSONComparer assertjson.Comparer

	varPrefix  string
	generators map[string]func() (interface{}, error)
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

	rv, err := replaceString(value, v)
	if err != nil {
		return ctx, fmt.Errorf("replacing vars in %s: %w", value, err)
	}

	val, gen, err := s.gen(value)
	if err != nil {
		return ctx, err
	}

	if !gen {
		if err := json.Unmarshal([]byte(rv), &val); err != nil {
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

	rv, err := replaceString(value, v)
	if err != nil {
		return fmt.Errorf("replacing vars in %s: %w", value, err)
	}

	stored, found := v.Get(s.varPrefix + name)
	if !found {
		return fmt.Errorf("could not find variable %s", name)
	}

	if err := assertjson.FailNotEqualMarshal([]byte(rv), stored); err != nil {
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

		value, err := replaceString(value, v)
		if err != nil {
			return ctx, fmt.Errorf("replacing vars in %s: %w", row.Cells[1].Value, err)
		}

		val, gen, err := s.gen(value)
		if err != nil {
			return ctx, err
		}

		if !gen {
			if err := json.Unmarshal([]byte(value), &val); err != nil {
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

		value, err := replaceString(value, v)
		if err != nil {
			return fmt.Errorf("failed to replace vars in %s: %w", row.Cells[1].Value, err)
		}

		stored, found := v.Get(name)
		if !found {
			return fmt.Errorf("could not find variable %s", name)
		}

		if err := assertjson.FailNotEqualMarshal([]byte(value), stored); err != nil {
			return fmt.Errorf("variable %s assertion failed: %w", name, err)
		}
	}

	return nil
}

func replaceString(s string, vars *shared.Vars) (string, error) {
	if vars != nil {
		type kv struct {
			k string
			v string
		}

		vv := vars.GetAll()
		kvs := make([]kv, 0, len(vv))

		for k, v := range vv {
			vs, err := json.Marshal(v)
			if err != nil {
				return "", fmt.Errorf("failed to marshal var %s (%v): %w", k, v, err)
			}

			if vs[0] == '"' {
				vs = bytes.Trim(vs, `"`)
			}

			kvs = append(kvs, kv{k: k, v: string(vs)})
		}

		sort.Slice(kvs, func(i, j int) bool {
			return len(kvs[i].k) > len(kvs[j].k)
		})

		for _, kv := range kvs {
			s = strings.ReplaceAll(s, kv.k, kv.v)
		}
	}

	return s, nil
}
