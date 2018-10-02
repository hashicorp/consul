@setupApplicationTest
Feature: dc / acls / policies / view managment: Readonly management policy
  Scenario:
    Given 1 datacenter model with the value "datacenter"
    And 1 policy model from yaml
    ---
      ID: 00000000-0000-0000-0000-000000000001
    ---
    When I visit the policy page for yaml
    ---
      dc: datacenter
      policy: 00000000-0000-0000-0000-000000000001
    ---
    Then the url should be /datacenter/acls/policies/00000000-0000-0000-0000-000000000001
    Then I see the text "View Policy" in "h1"
@ignore
  Scenario: Check the rest of the view policy content is correct
    Then ok

