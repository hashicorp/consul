@setupApplicationTest
Feature: dc / acls / roles / update: ACL Role Update
  Background:
    Given 1 datacenter model with the value "datacenter"
    And 1 role model from yaml
    ---
      ID: role-id
    ---
    And 3 token models
    When I visit the role page for yaml
    ---
      dc: datacenter
      role: role-id
    ---
    Then the url should be /datacenter/acls/roles/role-id
    Then I see 3 token models
    And the title should be "Edit Role - Consul"
  Scenario: Update to [Name], [Rules], [Description]
    Then I fill in the role form with yaml
    ---
      Name: [Name]
      Description: [Description]
    ---
    And I submit
    Then a PUT request was made to "/v1/acl/role/role-id?dc=datacenter&ns=@namespace" from yaml
    ---
      body:
        Name: [Name]
        Description: [Description]
    ---
    Then the url should be /datacenter/acls/roles
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
    Where:
      ------------------------------------------
      | Name        | Description              |
      | role-name   | role-name description    |
      | role        | role name description    |
      | roleName    | role%20name description  |
      ------------------------------------------
  Scenario: There was an error saving the key
    Given the url "/v1/acl/role/role-id" responds with a 500 status
    And I submit
    Then the url should be /datacenter/acls/roles/role-id
    Then "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "error" class

  @notNamespaceable
  Scenario: Updating a simple ACL role when Namespaces are disabled does not send Namespace
    Then I fill in the role form with yaml
    ---
      Description: Description
    ---
    And I submit
    Then a PUT request was made to "/v1/acl/role/role-id?dc=datacenter" without properties from yaml
    ---
      - Namespace
    ---
    Then the url should be /datacenter/acls/roles
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
