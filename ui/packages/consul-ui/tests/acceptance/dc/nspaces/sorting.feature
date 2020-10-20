@setupApplicationTest
@onlyNamespaceable
Feature: dc / nspaces / sorting
  Scenario: Sorting Namespaces
    Given settings from yaml
    ---
    consul:token:
      SecretID: secret
      AccessorID: accessor
      Namespace: default
    ---
    Given 1 datacenter model with the value "dc-1"
    And 6 nspace models from yaml
    ---
    - Name: "nspace-5"
    - Name: "nspace-3"
    - Name: "nspace-1"
    - Name: "nspace-4"
    - Name: "nspace-2"
    - Name: "nspace-6"
    ---
    When I visit the nspaces page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/namespaces
    Then I see 6 nspace models
    When I click selected on the sort
    When I click options.1.button on the sort
    Then I see name on the nspaces vertically like yaml
    ---
    - "nspace-6"
    - "nspace-5"
    - "nspace-4"
    - "nspace-3"
    - "nspace-2"
    - "nspace-1"
    ---
    When I click selected on the sort
    When I click options.0.button on the sort
    Then I see name on the nspaces vertically like yaml
    ---
    - "nspace-1"
    - "nspace-2"
    - "nspace-3"
    - "nspace-4"
    - "nspace-5"
    - "nspace-6"
    ---