@setupApplicationTest
Feature: dc / acls / tokens / index: ACL Token List

  Scenario: I see the tokens
    Given 1 datacenter model with the value "dc-1"
    And 3 token models
    When I visit the tokens page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/tokens
    Then I see 3 token models
  Scenario: I see the legacy message if I have one legacy token
    Given 1 datacenter model with the value "dc-1"
    And 3 token models from yaml
    ---
    - Legacy: true
    - Legacy: false
    - Legacy: false
    ---
    When I visit the tokens page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/tokens
    And I see update
    And I see 3 token models
  Scenario: I don't see the legacy message if I have no legacy tokens
    Given 1 datacenter model with the value "dc-1"
    And 3 token models from yaml
    ---
    - Legacy: false
    - Legacy: false
    - Legacy: false
    ---
    When I visit the tokens page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/tokens
    And I don't see update
    And I see 3 token models
