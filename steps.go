package vars

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/cucumber/godog"
	"github.com/swaggest/assertjson"
)

// Steps provides godog gherkin step definitions.
type Steps struct {
	JSONComparer assertjson.Comparer

	mu         sync.Mutex
	varPrefix  string
	generators map[string]func() (interface{}, error)

	globalVars  map[string]interface{}
	featureVars map[string]map[string]interface{}
}

// AddGenerator registers user-defined generator function, suitable for random identifiers.
func (s *Steps) AddGenerator(name string, f func() (interface{}, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.generators == nil {
		s.generators = make(map[string]func() (interface{}, error))
	}

	s.generators[name] = f
}

type fvCtxKey struct{}

// Register add steps to scenario context.
func (s *Steps) Register(sc *godog.ScenarioContext) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.varPrefix = "$"
	if s.JSONComparer.Vars != nil && s.JSONComparer.Vars.VarPrefix != "" {
		s.varPrefix = s.JSONComparer.Vars.VarPrefix
	}

	if s.globalVars == nil {
		s.globalVars = make(map[string]interface{})
	}

	sc.Before(s.setupGlobals)

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

	//    When variables are set to values if undefined in this feature
	//      | $bar   | "abc"             |
	//      | $baz   | {"one":1,"two":2} |
	sc.Step(`^variables are set to values once in this feature$`, s.varsAreSetOnceInThisFeature)

	//    When variables are set to values if undefined globally
	//      | $bar   | "abc"             |
	//      | $baz   | {"one":1,"two":2} |
	sc.Step(`^variables are set to values once globally$`, s.varsAreSetOnceGlobally)

	//    Then variable $bar matches JSON paths
	//      | $.foo          | "abcdef"   |
	//      | $.bar          | 123        |
	//      | $.baz          | true       |
	//      | $.prefixed_foo | "ooo::$foo" |
	sc.Step(`^variable \`+s.varPrefix+`([\w\d]+) matches JSON paths$`, s.varMatchesJSONPaths)
}

func (s *Steps) setupGlobals(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.featureVars == nil {
		s.featureVars = make(map[string]map[string]interface{})
	}

	fv := s.featureVars[sc.Uri]
	if fv == nil {
		fv = make(map[string]interface{})
		s.featureVars[sc.Uri] = fv
	}

	ctx = context.WithValue(ctx, fvCtxKey{}, fv)

	if len(fv) == 0 && len(s.globalVars) == 0 {
		return ctx, nil
	}

	ctx, v := s.JSONComparer.Vars.Fork(ctx)

	for key, val := range s.globalVars {
		v.Set(key, val)
	}

	for key, val := range fv {
		v.Set(key, val)
	}

	return ctx, nil
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

func (s *Steps) walkVars(ctx context.Context, table *godog.Table, override map[string]interface{}, cb func(name string, val interface{})) error {
	for _, row := range table.Rows {
		if len(row.Cells) != 2 {
			return fmt.Errorf("two columns expected in the table, %d received", len(row.Cells))
		}

		name := row.Cells[0].Value
		value := row.Cells[1].Value

		if v, found := override[name]; found {
			cb(name, v)

			continue
		}

		_, rv, err := s.Replace(ctx, []byte(value))
		if err != nil {
			return fmt.Errorf("replacing vars in %s: %w", row.Cells[1].Value, err)
		}

		val, gen, err := s.gen(string(rv))
		if err != nil {
			return err
		}

		if !gen {
			if err := json.Unmarshal(rv, &val); err != nil {
				return fmt.Errorf("decoding variable %s with value %s as JSON: %w", name, value, err)
			}
		}

		cb(name, val)
	}

	return nil
}

func (s *Steps) varsAreSetOnceInThisFeature(ctx context.Context, table *godog.Table) (context.Context, error) {
	ctx, v := s.Vars(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	fv, ok := ctx.Value(fvCtxKey{}).(map[string]interface{})
	if !ok {
		return ctx, fmt.Errorf("BUG: missing feature vars in context")
	}

	err := s.walkVars(ctx, table, fv, func(name string, val interface{}) {
		fv[name] = val
		v.Set(name, val)
	})

	return ctx, err
}

func (s *Steps) varsAreSetOnceGlobally(ctx context.Context, table *godog.Table) (context.Context, error) {
	ctx, v := s.Vars(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.walkVars(ctx, table, s.globalVars, func(name string, val interface{}) {
		s.globalVars[name] = val
		v.Set(name, val)
	})

	return ctx, err
}

func (s *Steps) varsAreSet(ctx context.Context, table *godog.Table) (context.Context, error) {
	ctx, v := s.Vars(ctx)

	err := s.walkVars(ctx, table, nil, func(name string, val interface{}) {
		v.Set(name, val)
	})

	return ctx, err
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
