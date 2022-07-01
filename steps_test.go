package vars_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cucumber/godog"
	"github.com/godogx/vars"
	"github.com/stretchr/testify/assert"
)

func TestFeatures(t *testing.T) {
	vs := vars.Steps{}

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
