@setupApplicationTest
Feature: dc / tokens / navigation
  Scenario: Clicking a token in the listing and back again
    Given 1 datacenter model with the value "dc-1"
    And 3 token models
    When I visit the tokens page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/tokens
    And the title should be "Tokens - Consul"
    Then I see 3 token models
    When I click token on the tokens
    And I click "[data-test-back]"
    Then the url should be /dc-1/acls/tokens
