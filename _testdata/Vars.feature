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
    When variables are set to values
      | $bar  | "abc"             |
      | $baz  | {"one":1,"two":2} |
      | $qux  | 123               |
      | $quux | true              |
    # Assert current values of multiple variables.
    Then variables are equal to values
      | $bar  | "abc"             |
      | $baz  | {"one":1,"two":2} |
      | $qux  | 123               |
      | $quux | true              |
    And variable $qux equals to 123


    # Use vars in custom steps.
    When I do foo
    Then foo is done
