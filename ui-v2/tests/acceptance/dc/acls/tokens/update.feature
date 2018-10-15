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
  Scenario: Update to [Name]
    Then I fill in with yaml
    ---
      Description: [Description]
    ---
    And I submit
    Then a PUT request is made to "/v1/acl/token/key?dc=datacenter" with the body from yaml
    ---
      Description: [Description]
    ---
    Then the url should be /datacenter/acls/tokens
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
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
    Then "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "error" class
