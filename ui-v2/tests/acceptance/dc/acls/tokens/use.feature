@setupApplicationTest
Feature: dc / acls / tokens / use: Using an ACL token
  Background:
    Given 1 datacenter model with the value "datacenter"
    And 1 token model from yaml
    ---
      AccessorID: token
    ---
  Scenario: Using an ACL token from the listing page
    When I visit the tokens page for yaml
    ---
      dc: datacenter
    ---
    Then I have settings like yaml
    ---
      token: ~
    ---
    And I click actions on the tokens
    And I click use on the tokens
    And I click confirmUse on the tokens
    Then "[data-notification]" has the "notification-use" class
    And "[data-notification]" has the "success" class
    Then I have settings like yaml
    ---
      token: token
    ---
  Scenario: Using an ACL token from the detail page
    When I visit the token page for yaml
    ---
      dc: datacenter
      token: token
    ---
    Then I have settings like yaml
    ---
      token: ~
    ---
    And I click use
    And I click confirmUse
    Then "[data-notification]" has the "notification-use" class
    And "[data-notification]" has the "success" class
    Then I have settings like yaml
    ---
      token: token
    ---
