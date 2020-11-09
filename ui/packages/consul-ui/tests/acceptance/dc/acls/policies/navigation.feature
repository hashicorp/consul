@setupApplicationTest
Feature: dc / policies / navigation
  Scenario: Clicking a policy in the listing and back again
    Given 1 datacenter model with the value "dc-1"
    And 3 policy models
    When I visit the policies page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/policies
    And the title should be "Policies - Consul"
    Then I see 3 policy models
    When I click policy on the policies
    And I click "[data-test-back]"
    Then the url should be /dc-1/acls/policies
