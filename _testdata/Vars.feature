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


    # Use vars in custom steps.
    When I do foo
    Then foo is done
