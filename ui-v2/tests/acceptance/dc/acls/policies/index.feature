@setupApplicationTest
Feature: dc / acls / policies / index: ACL Policy List

  Scenario:
    Given 1 datacenter model with the value "dc-1"
    And 3 policy models
    When I visit the policies page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/policies
    Then I see 3 policy models
@ignore
  Scenario: The global-managment policy can't be deleted
    And I click actions on the policies
    Then I don't see delete on the policies
