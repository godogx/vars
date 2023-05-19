Feature: feature 2

  Scenario Outline: setting vars
    Given variables are set to values once in this feature
      | $fv1 | gen:featureSeq |
      | $fv2 | gen:featureSeq |

    And variables are set to values once globally
      | $gv1 | gen:globalSeq |
      | $gv2 | gen:globalSeq |

    And variables are set to values
      | $lv1 | <lv1> |
      | $lv2 | <lv2> |

    Then variables are equal to values
      | $gv1 | 1     |
      | $gv2 | 2     |
      | $lv1 | <lv1> |
      | $lv2 | <lv2> |

    Examples:
      | lv1 | lv2 |
      | 3   | "a" |
      | 4   | "b" |
      | 5   | "c" |
      | 6   | "d" |
      | 7   | "e" |
