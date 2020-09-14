@setupApplicationTest
Feature: dc / acls / roles / sorting
  Scenario: Sorting Roles
    Given 1 datacenter model with the value "dc-1"
    And 4 role models from yaml
    ---
    - Name: "system-A"
      CreateIndex: 3
    - Name: "system-D"
      CreateIndex: 2
    - Name: "system-C"
      CreateIndex: 1
    - Name: "system-B"
      CreateIndex: 4
    ---
    When I visit the roles page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/roles
    Then I see 4 role models
    When I click selected on the sort
    When I click options.1.button on the sort
    Then I see name on the roles vertically like yaml
    ---
    - "system-D"
    - "system-C"
    - "system-B"
    - "system-A"
    ---
    When I click selected on the sort
    When I click options.0.button on the sort
    Then I see name on the roles vertically like yaml
    ---
    - "system-A"
    - "system-B"
    - "system-C"
    - "system-D"
    ---
    When I click selected on the sort
    When I click options.3.button on the sort
    Then I see name on the roles vertically like yaml
    ---
    - "system-C"
    - "system-D"
    - "system-A"
    - "system-B"
    ---
    When I click selected on the sort
    When I click options.2.button on the sort
    Then I see name on the roles vertically like yaml
    ---
    - "system-B"
    - "system-A"
    - "system-D"
    - "system-C"
    ---
