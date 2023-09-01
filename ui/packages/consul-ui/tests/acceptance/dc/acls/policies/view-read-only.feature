@setupApplicationTest
Feature: dc / acls / policies / view read-only policy: Readonly management policy
  Background:
    Given 1 datacenter model with the value "datacenter"
    And 1 policy model from yaml
    ---
      ID: 00000000-0000-0000-0000-000000000002
    ---
  Scenario:
    When I visit the policy page for yaml
    ---
      dc: datacenter
      policy: 00000000-0000-0000-0000-000000000002
    ---
    Then the url should be /datacenter/acls/policies/00000000-0000-0000-0000-000000000002
    Then I see the text "View Policy" in "h1"
    Then I don't see confirmDelete
    Then I don't see cancel
    And I see tokens

