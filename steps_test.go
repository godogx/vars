package vars_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/godogx/vars"
	"github.com/stretchr/testify/assert"
)

func TestFeatures(t *testing.T) { //nolint:cyclop
	vs := vars.Steps{}
	vs.AddGenerator("new-id", func() (interface{}, error) {
		return 1337, nil
	})

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
		return ctx, 12321, nil
	})

	suite := godog.TestSuite{}
	suite.ScenarioInitializer = func(s *godog.ScenarioContext) {
		vs.Register(s)

		// Other configuration and step definitions.

		// Libraries, that are unaware of each other, can use vars to communicate general state between themselves.
		s.Step("^I do foo$", func(ctx context.Context) context.Context {
			return vars.ToContext(ctx, "$fooDone", true)
		})

		s.Step("^foo is done$", func(ctx context.Context) error {
			if done, ok := vars.FromContext(ctx)["$fooDone"]; ok {
				if b, ok := done.(bool); ok && b {
					return nil
				}
			}

			return errors.New("foo is not done")
		})
	}

	suite.Options = &godog.Options{
		Format:   "pretty",
		Strict:   true,
		Paths:    []string{"_testdata/Vars.feature"},
		TestingT: t,
	}

	assert.Zero(t, suite.Run(), "suite failed")
}

func TestFeatures_global(t *testing.T) {
	var (
		featureSeq int64
		globalSeq  int64
	)

	vs := vars.Steps{}

	vs.AddGenerator("featureSeq", func() (interface{}, error) {
		v := atomic.AddInt64(&featureSeq, 1)
		assert.Less(t, v, int64(5))

		return v, nil
	})

	vs.AddGenerator("globalSeq", func() (interface{}, error) {
		v := atomic.AddInt64(&globalSeq, 1)
		assert.Less(t, v, int64(3))

		return v, nil
	})

	suite := godog.TestSuite{}
	suite.ScenarioInitializer = func(s *godog.ScenarioContext) {
		vs.Register(s)
	}

	suite.Options = &godog.Options{
		Format:      "pretty",
		Strict:      true,
		Paths:       []string{"_testdata/Feature1.feature", "_testdata/Feature2.feature"},
		Concurrency: 10,
		TestingT:    t,
	}

	assert.Zero(t, suite.Run(), "suite failed")
}
