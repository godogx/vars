package vars

import (
	"context"
	"encoding/json"
	"fmt"

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

	varPrefix string
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
	//      | $bar  | "abc"             |
	//      | $baz  | {"one":1,"two":2} |
	//      | $qux  | 123               |
	//      | $quux | true              |
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
	var val interface{}
	if err := json.Unmarshal([]byte(value), &val); err != nil {
		return ctx, fmt.Errorf("failed to decode variable %s with value %s as JSON: %w", name, value, err)
	}

	ctx, v := s.JSONComparer.Vars.Fork(ctx)
	v.Set(s.varPrefix+name, val)

	return ctx, nil
}

func (s *Steps) varEquals(ctx context.Context, name, value string) error {
	_, v := s.JSONComparer.Vars.Fork(ctx)

	stored, found := v.Get(s.varPrefix + name)
	if !found {
		return fmt.Errorf("could not find variable %s", name)
	}

	if err := assertjson.FailNotEqualMarshal([]byte(value), stored); err != nil {
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

		var val interface{}
		if err := json.Unmarshal([]byte(value), &val); err != nil {
			return ctx, fmt.Errorf("failed to decode variable %s with value %s as JSON: %w", name, value, err)
		}

		v.Set(s.varPrefix+name, val)
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

		stored, found := v.Get(s.varPrefix + name)
		if !found {
			return fmt.Errorf("could not find variable %s", name)
		}

		if err := assertjson.FailNotEqualMarshal([]byte(value), stored); err != nil {
			return fmt.Errorf("variable %s assertion failed: %w", name, err)
		}
	}

	return nil
}
