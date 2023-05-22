package vars_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/cucumber/godog"
	"github.com/godogx/vars"
)

func ExampleAssert() {
	// Variables are passed via chained context.
	ctx := context.Background()

	expected := []byte(`{"foo":"$foo","bar":123}`)
	received := []byte(`{"foo":321,"bar":123,"baz":true}`)

	// No error, $foo is populated with 321, "baz" is ignored for true `ignoreAddedJSONFields` argument.
	ctx, err := vars.Assert(ctx, expected, received, true)
	if err != nil {
		fmt.Println("assertion failed: " + err.Error())
	}

	expected = []byte(`{"foo":"$foo","bar":123,"prefixed_foo":"ooo::$foo"}`)
	received = []byte(`{"foo":313,"bar":123,"baz":true,"prefixed_foo":"ooo::321"}`)
	// Assertion fails.
	_, err = vars.Assert(ctx, expected, received, false)
	if err != nil {
		fmt.Println("assertion failed: " + err.Error())
	}

	// Output:
	// assertion failed: not equal:
	//  {
	//    "bar": 123,
	// -  "foo": 321,
	// +  "foo": 313,
	//    "prefixed_foo": "ooo::321"
	// +  "baz": true
	//  }
}

func ExampleReplace() {
	// Variables are passed via chained context.
	ctx := context.Background()

	ctx = vars.ToContext(ctx, "$foo", 321)

	expected := []byte(`{"foo":"$foo","bar":123, "prefixed_foo":"ooo::$foo"}`)

	_, expected, err := vars.Replace(ctx, expected)
	if err != nil {
		fmt.Println("replace failed: " + err.Error())
	}

	fmt.Println(string(expected))

	// Output:
	// {"foo":321,"bar":123,"prefixed_foo":"ooo::321"}
}

func ExampleSteps_AddFactory() {
	vs := &vars.Steps{}

	vs.AddFactory("now", func(ctx context.Context, args ...interface{}) (context.Context, interface{}, error) {
		// "Now" is mocked with a constant value to reproducibility.
		return ctx, time.Date(2023, 5, 22, 19, 38, 0, 0, time.UTC), nil
	})

	vs.AddFactory("addDuration", func(ctx context.Context, args ...interface{}) (context.Context, interface{}, error) {
		if len(args) != 2 {
			return ctx, nil, errors.New("addDuration expects 2 arguments: base time, duration")
		}
		var (
			base time.Time
			dur  time.Duration
		)

		switch v := args[0].(type) {
		case time.Time:
			base = v
		case string:
			t, err := time.Parse(time.RFC3339Nano, v)
			if err != nil {
				return ctx, nil, fmt.Errorf("parsing base time: %w", err)
			}
			base = t
		default:
			return ctx, nil, fmt.Errorf("unexpected type %T for base time, string or time.Time expected", v)
		}

		switch v := args[1].(type) {
		case time.Duration:
			dur = v
		case string:
			d, err := time.ParseDuration(v)
			if err != nil {
				return ctx, nil, fmt.Errorf("parsing duration: %w", err)
			}
			dur = d
		default:
			return ctx, nil, fmt.Errorf("unexpected type %T for duration, string or time.Duration expected", v)
		}

		return ctx, base.Add(dur), nil
	})

	vs.AddFactory("newUserID", func(ctx context.Context, args ...interface{}) (context.Context, interface{}, error) {
		if len(args) != 2 {
			return ctx, nil, errors.New("newUserID expects 2 arguments: name, registeredAt")
		}
		var (
			name         string
			registeredAt time.Time
		)

		switch v := args[0].(type) {
		case string:
			name = v
		default:
			return ctx, nil, fmt.Errorf("unexpected type %T for name, string expected", v)
		}

		switch v := args[1].(type) {
		case time.Time:
			registeredAt = v
		case string:
			t, err := time.Parse(time.RFC3339Nano, v)
			if err != nil {
				return ctx, nil, fmt.Errorf("parsing registeredAt: %w", err)
			}
			registeredAt = t
		default:
			return ctx, nil, fmt.Errorf("unexpected type %T for registeredAt, string or time.Time expected", v)
		}

		fmt.Println("creating user", name, registeredAt)

		// Return relevant value, for example user id.
		return ctx, 123, nil
	})

	s := godog.TestSuite{}

	s.ScenarioInitializer = func(sc *godog.ScenarioContext) {
		vs.Register(sc)
	}

	s.Options = &godog.Options{
		Format: "pretty",
		Output: io.Discard,
		FeatureContents: []godog.Feature{
			{
				Name: "example",
				Contents: []byte(`
Feature: example
Scenario: using var factory
   Given variable $myUserID is set to newUserID("John Doe", addDuration(now(), "-10h"))
`),
			},
		},
	}

	s.Run()

	// Output:
	// creating user John Doe 2023-05-22 09:38:00 +0000 UTC
}
