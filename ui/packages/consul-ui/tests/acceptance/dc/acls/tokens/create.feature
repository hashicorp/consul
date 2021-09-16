@setupApplicationTest
Feature: dc / acls / tokens / create
  Background:
    Given 1 datacenter model with the value "datacenter"
    When I visit the token page for yaml
    ---
      dc: datacenter
    ---
  Scenario: Visiting the page without error and the title is correct
    Then the url should be /datacenter/acls/tokens/create
    And the title should be "New Token - Consul"
  Scenario: Creating a simple ACL token with description [Description]
    Then I fill in with yaml
    ---
      Description: [Description]
    ---
    And I submit
    Then a PUT request was made to "/v1/acl/token?dc=datacenter&ns=@namespace" from yaml
    ---
      body:
        Description: [Description]
    ---
    Then the url should be /datacenter/acls/tokens
    And "[data-notification]" has the "notification-create" class
    And "[data-notification]" has the "success" class
    Where:
      ---------------------------
      | Description             |
      | description             |
      | description with spaces |
      ---------------------------
  @notNamespaceable
  Scenario: Creating a simple ACL token when Namespaces are disabled does not send Namespace
    Then I fill in with yaml
    ---
      Description: Description
    ---
    And I submit
    Then a PUT request was made to "/v1/acl/token?dc=datacenter" without properties from yaml
    ---
      - Namespace
    ---
    Then the url should be /datacenter/acls/tokens
    And "[data-notification]" has the "notification-create" class
    And "[data-notification]" has the "success" class
