@setupApplicationTest
Feature: dc / acls / use: Using an ACL token
  Background:
    Given 1 datacenter model with the value "datacenter"
    And I'm using a legacy token
    And 1 acl model from yaml
    ---
      ID: token
    ---
  Scenario: Using an ACL token from the listing page
    When I visit the acls page for yaml
    ---
      dc: datacenter
    ---
    Then I have settings like yaml
    ---
    consul:token: '{"AccessorID":null,"SecretID":"id"}'
    ---
    And I click actions on the acls
    And I click use on the acls
    And I click confirmUse on the acls
    Then "[data-notification]" has the "notification-use" class
    And "[data-notification]" has the "success" class
    Then I have settings like yaml
    ---
    consul:token: '{"AccessorID":null,"SecretID":"token"}'
    ---
  Scenario: Using an ACL token from the detail page
    When I visit the acl page for yaml
    ---
      dc: datacenter
      acl: token
    ---
    Then I have settings like yaml
    ---
    consul:token: '{"AccessorID":null,"SecretID":"id"}'
    ---
    And I click use
    And I click confirmUse
    Then "[data-notification]" has the "notification-use" class
    And "[data-notification]" has the "success" class
    Then I have settings like yaml
    ---
    consul:token: '{"AccessorID":null,"SecretID":"token"}'
    ---
