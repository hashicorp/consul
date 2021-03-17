@setupApplicationTest
Feature: dc / acls / auth-methods / navigation
  Scenario: Clicking a auth-method in the listing and back again
    Given 1 datacenter model with the value "dc-1"
    And 3 authMethod models
    When I visit the authMethods page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/auth-methods
    And the title should be "Auth Methods - Consul"
    Then I see 3 authMethod models
    When I click authMethod on the authMethods
    And I click "[data-test-back]"
    Then the url should be /dc-1/acls/auth-methods

