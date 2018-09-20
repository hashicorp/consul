@setupApplicationTest
Feature: dc / kvs / update: KV Update
  Background:
    Given 1 datacenter model with the value "datacenter"
  Scenario: Update to [Name] change value to [Value]
    And 1 kv model from yaml
    ---
      Key: [Name]
    ---
    When I visit the kv page for yaml
    ---
      dc: datacenter
      kv: [Name]
    ---
    Then the url should be /datacenter/kv/[Name]/edit
    # Turn the Code Editor off so we can fill the value easier
    And I click "[name=json]"
    Then I fill in with yaml
    ---
      value: [Value]
    ---
    And I submit
    Then a PUT request is made to "/v1/kv/[Name]?dc=datacenter" with the body "[Value]"
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
  Where:
      --------------------------------------------
      | Name                      | Value        |
      | key                       | value        |
      | key-name                  | a value      |
      | folder/key-name           | a value      |
      --------------------------------------------
  Scenario: Update to a key change value to '   '
    And 1 kv model from yaml
    ---
      Key: key
    ---
    When I visit the kv page for yaml
    ---
      dc: datacenter
      kv: key
    ---
    Then the url should be /datacenter/kv/key/edit
    # Turn the Code Editor off so we can fill the value easier
    And I click "[name=json]"
    Then I fill in with yaml
    ---
      value: '   '
    ---
    And I submit
    Then a PUT request is made to "/v1/kv/key?dc=datacenter" with the body "   "
    Then the url should be /datacenter/kv
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
  Scenario: Update to a key change value to ''
    And 1 kv model from yaml
    ---
      Key: key
    ---
    When I visit the kv page for yaml
    ---
      dc: datacenter
      kv: key
    ---
    Then the url should be /datacenter/kv/key/edit
    # Turn the Code Editor off so we can fill the value easier
    And I click "[name=json]"
    Then I fill in with yaml
    ---
      value: ''
    ---
    And I submit
    Then a PUT request is made to "/v1/kv/key?dc=datacenter" with no body
    Then the url should be /datacenter/kv
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
  Scenario: Update to a key when the value is empty
    And 1 kv model from yaml
    ---
    Key: key
    Value: ~
    ---
    When I visit the kv page for yaml
    ---
      dc: datacenter
      kv: key
    ---
    Then the url should be /datacenter/kv/key/edit
    And I submit
    Then a PUT request is made to "/v1/kv/key?dc=datacenter" with no body
    Then the url should be /datacenter/kv
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
  Scenario: There was an error saving the key
    When I visit the kv page for yaml
    ---
      dc: datacenter
      kv: key
    ---
    Then the url should be /datacenter/kv/key/edit

    Given the url "/v1/kv/key" responds with a 500 status
    And I submit
    Then the url should be /datacenter/kv/key/edit
    Then "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "error" class
@ignore
  Scenario: KV's with spaces are saved correctly
    Then ok
@ignore
  Scenario: KV's with returns are saved correctly
    Then ok
