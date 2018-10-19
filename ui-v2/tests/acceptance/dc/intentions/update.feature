@setupApplicationTest
Feature: dc / intentions / update: Intention Update
  Background:
    Given 1 datacenter model with the value "datacenter"
    And 1 intention model from yaml
    ---
      ID: intention-id
    ---
    When I visit the intention page for yaml
    ---
      dc: datacenter
      intention: intention-id
    ---
    Then the url should be /datacenter/intentions/intention-id
  Scenario: Update to [Description], [Action]
    Then I fill in with yaml
    ---
      Description: [Description]
    ---
    And I click "[value=[Action]]"
    And I submit
    Then a PUT request is made to "/v1/connect/intentions/intention-id?dc=datacenter" with the body from yaml
    ---
      Description: [Description]
      Action: [Action]
    ---
    Then the url should be /datacenter/intentions
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
    Where:
      ------------------------------
      | Description       | Action |
      | Desc              | allow  |
      ------------------------------
  Scenario: There was an error saving the intention
    Given the url "/v1/connect/intentions/intention-id" responds with a 500 status
    And I submit
    Then the url should be /datacenter/intentions/intention-id
    Then "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "error" class

