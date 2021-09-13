@setupApplicationTest
Feature: dc / kvs / update: KV Update
  Background:
    Given 1 datacenter model with the value "datacenter"
  Scenario: Update to [Name] change value to [Value]
    And 1 kv model from yaml
    ---
      Key: "[Name]"
      Flags: 12
    ---
    When I visit the kv page for yaml
    ---
      dc: datacenter
      kv: "[Name]"
    ---
    Then the url should be /datacenter/kv/[EncodedName]/edit
    And the title should be "Edit Key / Value - Consul"
    # Turn the Code Editor off so we can fill the value easier
    And I click "[name=json]"
    Then I fill in with yaml
    ---
      value: [Value]
    ---
    And I submit
    Then a PUT request was made to "/v1/kv/[EncodedName]?dc=datacenter&ns=@!namespace&flags=12" with the body "[Value]"
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
  Where:
      ---------------------------------------------------------
      | Name            | EncodedName          | Value        |
      | key             | key                  | value        |
      | #key            | %23key               | value        |
      | key-name        | key-name             | a value      |
      | key name        | key%20name           | a value      |
      | folder/key-name | folder/key-name      | a value      |
      ---------------------------------------------------------
  Scenario: Update to a key change value to '   '
    And 1 kv model from yaml
    ---
      Key: key
      Flags: 12
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
    Then a PUT request was made to "/v1/kv/key?dc=datacenter&ns=@!namespace&flags=12" with the body "   "
    Then the url should be /datacenter/kv
    And the title should be "Key / Value - Consul"
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
  Scenario: Update to a key change value to ''
    And 1 kv model from yaml
    ---
      Key: key
      Flags: 12
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
    Then a PUT request was made to "/v1/kv/key?dc=datacenter&ns=@!namespace&flags=12" with no body
    Then the url should be /datacenter/kv
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
  Scenario: Update to a key when the value is empty
    And 1 kv model from yaml
    ---
      Key: key
      Value: ~
      Flags: 12
    ---
    When I visit the kv page for yaml
    ---
      dc: datacenter
      kv: key
    ---
    Then the url should be /datacenter/kv/key/edit
    And I submit
    Then a PUT request was made to "/v1/kv/key?dc=datacenter&ns=@!namespace&flags=12" with no body
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
