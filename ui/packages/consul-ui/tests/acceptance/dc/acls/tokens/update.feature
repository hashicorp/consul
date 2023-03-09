@setupApplicationTest
Feature: dc / acls / tokens / update: ACL Token Update
  Background:
    Given 1 datacenter model with the value "datacenter"
    And 1 token model from yaml
    ---
      AccessorID: key
      Policies: ~
    ---
    When I visit the token page for yaml
    ---
      dc: datacenter
      token: key
    ---
    Then the url should be /datacenter/acls/tokens/key
    And the title should be "Edit Token - Consul"
  Scenario: Update to [Name]
    Then I fill in with yaml
    ---
      Description: [Description]
    ---
    And I submit
    Then a PUT request was made to "/v1/acl/token/key?dc=datacenter&ns=@!namespace" from yaml
    ---
      body:
        Description: [Description]
    ---
    Then the url should be /datacenter/acls/tokens
    And "[data-notification]" has the "hds-toast" class
    And "[data-notification]" has the "hds-alert--color-success" class
    Where:
      ---------------------------
      | Description             |
      | description             |
      | description with spaces |
      ---------------------------
  Scenario: There was an error saving the key
    Given the url "/v1/acl/token/key" responds with a 500 status
    And I submit
    Then the url should be /datacenter/acls/tokens/key
    And "[data-notification]" has the "hds-toast" class
    And "[data-notification]" has the "hds-alert--color-critical" class

  @notNamespaceable
  Scenario: Updating a simple ACL token when Namespaces are disabled does not send Namespace
    Then I fill in with yaml
    ---
      Description: Description
    ---
    And I submit
    Then a PUT request was made to "/v1/acl/token/key?dc=datacenter" without properties from yaml
    ---
      - Namespace
    ---
    Then the url should be /datacenter/acls/tokens
    And "[data-notification]" has the "hds-toast" class
    And "[data-notification]" has the "hds-alert--color-success" class
