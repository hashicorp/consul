@setupApplicationTest
Feature: dc / acls / policies / update: ACL Policy Update
  Background:
    Given 1 datacenter model with the value "datacenter"
    And 1 policy model from yaml
    ---
      ID: policy-id
      Datacenters: []
    ---
    And 3 token models
    When I visit the policy page for yaml
    ---
      dc: datacenter
      policy: policy-id
    ---
    Then the url should be /datacenter/acls/policies/policy-id
    Then I see 3 token models
    And the title should be "Edit Policy - Consul"
  Scenario: Update to [Name], [Rules], [Description]
    Then I fill in the policy form with yaml
    ---
      Name: [Name]
      Description: [Description]
      Rules: [Rules]
    ---
    And I click validDatacenters
    And I click datacenter
    And I submit
    Then a PUT request was made to "/v1/acl/policy/policy-id?dc=datacenter&ns=@!namespace" from yaml
    ---
      body:
        Name: [Name]
        Description: [Description]
        Rules: [Rules]
        Datacenters:
          - datacenter

    ---
    Then the url should be /datacenter/acls/policies
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
    Where:
      ------------------------------------------------------------------------------
      | Name          |  Rules                        | Description                |
      | policy-name   |  key "foo" {policy = "read"}  | policy-name description    |
      | policy_name   |  key "foo" {policy = "write"} | policy name description    |
      | policyName    |  key "foo" {policy = "read"}  | policy%20name description  |
      ------------------------------------------------------------------------------
  Scenario: There was an error saving the key
    Given the url "/v1/acl/policy/policy-id" responds with a 500 status
    And I submit
    Then the url should be /datacenter/acls/policies/policy-id
    Then "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "error" class

  @notNamespaceable
  Scenario: Updating a simple ACL policy when Namespaces are disabled does not send Namespace
    Then I fill in the policy form with yaml
    ---
      Description: Description
    ---
    And I submit
    Then a PUT request was made to "/v1/acl/policy/policy-id?dc=datacenter" without properties from yaml
    ---
      - Namespace
    ---
    Then the url should be /datacenter/acls/policies
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
