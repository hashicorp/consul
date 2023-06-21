@setupApplicationTest
Feature: dc / acls / roles / create
  Background:
    Given 1 datacenter model with the value "datacenter"
    When I visit the role page for yaml
    ---
      dc: datacenter
    ---

  Scenario: Visiting the page without error and the title is correct
    Then the url should be /datacenter/acls/roles/create
    And the title should be "New Role - Consul"
  Scenario: Creating a simple ACL role with description [Description]
    Then I fill in the role form with yaml
    ---
      Name: my-role
      Description: [Description]
    ---
    And I submit
    Then a PUT request was made to "/v1/acl/role?dc=datacenter&ns=@namespace" from yaml
    ---
      body:
        Name: my-role
        Description: [Description]
    ---
    Then the url should be /datacenter/acls/roles
    And "[data-notification]" has the "hds-toast" class
    And "[data-notification]" has the "hds-alert--color-success" class
    Where:
      ---------------------------
      | Description             |
      | description             |
      | description with spaces |
      ---------------------------
  @notNamespaceable
  Scenario: Creating a simple ACL role when Namespaces are disabled does not send Namespace
    Then I fill in the role form with yaml
    ---
      Name: my-role
      Description: Description
    ---
    And I submit
    Then a PUT request was made to "/v1/acl/role?dc=datacenter" without properties from yaml
    ---
      - Namespace
    ---
    Then the url should be /datacenter/acls/roles
    And "[data-notification]" has the "hds-toast" class
    And "[data-notification]" has the "hds-alert--color-success" class
