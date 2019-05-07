@setupApplicationTest
Feature: dc / acls / policies / delete: Policy Delete
  Background:
    Given 1 datacenter model with the value "datacenter"
  Scenario: Deleting a policy model from the policies listing page
    Given 1 policy model from yaml
    ---
      ID: 1981f51d-301a-497b-89a0-05112ef02b4b
    ---
    When I visit the policies page for yaml
    ---
      dc: datacenter
    ---
    And I click actions on the policies
    And I click delete on the policies
    And I click confirmDelete on the policies
    Then a DELETE request is made to "/v1/acl/policy/1981f51d-301a-497b-89a0-05112ef02b4b?dc=datacenter"
    And "[data-notification]" has the "notification-delete" class
    And "[data-notification]" has the "success" class
    Given the url "/v1/acl/policy/1981f51d-301a-497b-89a0-05112ef02b4b?dc=datacenter" responds with a 500 status
    And I click actions on the policies
    And I click delete on the policies
    And I click confirmDelete on the policies
    And "[data-notification]" has the "notification-delete" class
    And "[data-notification]" has the "error" class
  Scenario: Deleting a policy from the policy detail page
    When I visit the policy page for yaml
    ---
      dc: datacenter
      policy: 1981f51d-301a-497b-89a0-05112ef02b4b
    ---
    And I click delete
    And I click confirmDelete on the deleteModal
    Then a DELETE request is made to "/v1/acl/policy/1981f51d-301a-497b-89a0-05112ef02b4b?dc=datacenter"
    And "[data-notification]" has the "notification-delete" class
    And "[data-notification]" has the "success" class
    When I visit the policy page for yaml
    ---
      dc: datacenter
      policy: 1981f51d-301a-497b-89a0-05112ef02b4b
    ---
    Given the url "/v1/acl/policy/1981f51d-301a-497b-89a0-05112ef02b4b?dc=datacenter" responds with a 500 status
    And I click delete
    And I click confirmDelete on the deleteModal
    And "[data-notification]" has the "notification-delete" class
    And "[data-notification]" has the "error" class
