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
    Then I fill in with yaml
    ---
      value: [Value]
    ---
    And I submit
    Then a PUT request is made to "/v1/kv/[Name]?dc=datacenter" with the body "[Value]"
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
    Then I fill in with yaml
    ---
      value: '   '
    ---
    And I submit
    Then a PUT request is made to "/v1/kv/key?dc=datacenter" with the body "   "
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
    Then I fill in with yaml
    ---
      value: ''
    ---
    And I submit
    Then a PUT request is made to "/v1/kv/key?dc=datacenter" with no body
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
@ignore
  Scenario: The feedback dialog says success or failure
    Then ok
@ignore
  Scenario: KV's with spaces are saved correctly
    Then ok
@ignore
  Scenario: KV's with returns are saved correctly
    Then ok
