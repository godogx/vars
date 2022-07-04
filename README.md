# vars

[![Build Status](https://github.com/godogx/vars/workflows/test-unit/badge.svg)](https://github.com/godogx/vars/actions?query=branch%3Amaster+workflow%3Atest-unit)
[![Coverage Status](https://codecov.io/gh/godogx/vars/branch/master/graph/badge.svg)](https://codecov.io/gh/godogx/vars)
[![GoDevDoc](https://img.shields.io/badge/dev-doc-00ADD8?logo=go)](https://pkg.go.dev/github.com/godogx/vars)
[![Time Tracker](https://wakatime.com/badge/github/godogx/vars.svg)](https://wakatime.com/badge/github/godogx/vars)
![Code lines](https://sloc.xyz/github/godogx/vars/?category=code)
![Comments](https://sloc.xyz/github/godogx/vars/?category=comments)

This library provides [`godog`](https://github.com/cucumber/godog) step definitions to manage variables shared between 
steps and API for other libraries.

## Usage

Register steps to `godog` scenario context.

```go
vs := vars.Steps{}

suite := godog.TestSuite{}
suite.ScenarioInitializer = func(s *godog.ScenarioContext) {
    vs.Register(s)
    
    // Other configuration and step definitions.
}
```

Use steps in `feature` files.

```gherkin
Feature: Variables

  Scenario: Setting and asserting variables
    # Assert variable have not been set.
    # Every variable name starts with var prefix, "$" by default.
    Given variable $foo is undefined
    # Set assign value to variable.
    # Every value is declared as JSON.
    When variable $foo is set to "abcdef"
    # Assert current value of variable.
    Then variable $foo equals to "abcdef"

    # Set values to multiple variables.
    # Values are decoded into `any` with JSON decoder.
    # Beware that both integers and floats will be decoded as `float64`.
    # String values can interpolate other variables (see $replaced).
    When variables are set to values
      | $bar      | "abc"             |
      | $baz      | {"one":1,"two":2} |
      | $qux      | 123               |
      | $quux     | true              |
      | $replaced | "$qux/test/$bar"  |
    # Assert current values of multiple variables.
    # String values can interpolate other variables (see $replaced: "$qux/test/$bar").
    Then variables are equal to values
      | $bar      | "abc"             |
      | $baz      | {"one":1,"two":2} |
      | $qux      | 123               |
      | $quux     | true              |
      | $replaced | "123/test/abc"    |
      | $replaced | "$qux/test/$bar"  |
    And variable $qux equals to 123
    And variable $replaced equals to "$qux/test/$bar"
    And variable $replaced equals to "123/test/abc"
```

Libraries can pass variables using context.
For example [`httpsteps`](https://github.com/godogx/httpsteps) can set variable from API response and then 
[`dbsteps`](https://github.com/godogx/dbsteps) can use that value to query database.

```go
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

```

```gherkin
    # Use vars in custom steps.
    When I do foo
    Then foo is done
```