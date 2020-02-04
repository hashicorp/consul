@setupApplicationTest
@onlyNamespaceable
Feature: dc / nspaces / index: Nspaces List
  Background:
    Given settings from yaml
    ---
    consul:token:
      SecretID: secret
      AccessorID: accessor
      Namespace: default
    ---
    And 1 datacenter model with the value "dc-1"
    And 3 nspace models
    When I visit the nspaces page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/namespaces
    And the title should be "Namespaces - Consul"
  Scenario:
    Then I see 3 nspace models
  Scenario: Searching the nspaces
    Then I see 3 nspace models
    Then I fill in with yaml
    ---
    s: default
    ---
    And I see 1 nspace model
    And I see 1 nspace model with the description "The default namespace"
  Scenario: The default namespace can't be deleted
    Then I see 3 nspace models
    And I click actions on the nspaces
    Then I don't see delete on the nspaces
