@setupApplicationTest
Feature: dc / acls / tokens / policies: ACL Token add existing policy
  Scenario:
    Given 1 datacenter model with the value "datacenter"
    And 1 token model from yaml
    ---
      AccessorID: key
      Policies: ~
    ---
    When I visit the token page for yaml
    ---
      dc: datacenter
      token: key
    ---
    Then the url should be /datacenter/acls/tokens/key
    And I click "[data-test-policy-element] .ember-power-select-trigger"
    And I click ".ember-power-select-option:first-child"
    And I see 1 policy model
    And I click "[data-test-policy-element] .ember-power-select-trigger"
    And I click ".ember-power-select-option:nth-child(2)"
    And I see 2 policy models
