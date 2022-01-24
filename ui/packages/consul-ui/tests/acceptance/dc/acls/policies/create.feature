@setupApplicationTest
Feature: dc / acls / policies / create
  Background:
    Given 1 datacenter model with the value "datacenter"
    When I visit the policy page for yaml
    ---
      dc: datacenter
    ---
  Scenario: Visiting the page without error and the title is correct
    Then the url should be /datacenter/acls/policies/create
    And the title should be "New Policy - Consul"

  Scenario: Creating a simple ACL policy with description [Description]
    Then I fill in the policy form with yaml
    ---
      Name: my-policy
      Description: [Description]
    ---
    And I submit
    Then a PUT request was made to "/v1/acl/policy?dc=datacenter&ns=@namespace" from yaml
    ---
      body:
        Name: my-policy
        Description: [Description]
    ---
    Then the url should be /datacenter/acls/policies
    And "[data-notification]" has the "notification-create" class
    And "[data-notification]" has the "success" class
    Where:
      ---------------------------
      | Description             |
      | description             |
      | description with spaces |
      ---------------------------

  @notNamespaceable
  Scenario: Creating a simple ACL policy when Namespaces are disabled does not send Namespace
    Then I fill in the policy form with yaml
    ---
      Name: my-policy
      Description: Description
    ---
    And I submit
    Then a PUT request was made to "/v1/acl/policy?dc=datacenter" without properties from yaml
    ---
      - Namespace
    ---
    Then the url should be /datacenter/acls/policies
    And "[data-notification]" has the "notification-create" class
    And "[data-notification]" has the "success" class
