@setupApplicationTest
Feature: dc / roles / navigation
  Scenario: Clicking a role in the listing and back again
    Given 1 datacenter model with the value "dc-1"
    And 3 role models
    When I visit the roles page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/roles
    And the title should be "Roles - Consul"
    Then I see 3 role models
    When I click role on the roles
    And I click "[data-test-back]"
    Then the url should be /dc-1/acls/roles

