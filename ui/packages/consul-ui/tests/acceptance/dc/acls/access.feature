@setupApplicationTest
Feature: dc / acls / access: ACLs Access
  Scenario: ACLs are disabled
    Given ACLs are disabled
    And 1 datacenter model with the value "dc-1"
    When I visit the tokens page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/tokens
    And I see the "[data-test-acls-disabled]" element
