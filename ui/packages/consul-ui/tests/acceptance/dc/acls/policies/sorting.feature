@setupApplicationTest
Feature: dc / acls / policies / sorting
  Scenario: Sorting Policies
    Given 1 datacenter model with the value "dc-1"
    And 4 policy models from yaml
    ---
    - Name: "system-A"
    - Name: "system-D"
    - Name: "system-C"
    - Name: "system-B"
    ---
    When I visit the policies page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/policies
    Then I see 4 policy models
    When I click selected on the sort
    When I click options.1.button on the sort
    Then I see name on the policies vertically like yaml
    ---
    - "system-D"
    - "system-C"
    - "system-B"
    - "system-A"
    ---
    When I click selected on the sort
    When I click options.0.button on the sort
    Then I see name on the policies vertically like yaml
    ---
    - "system-A"
    - "system-B"
    - "system-C"
    - "system-D"
    ---

