@setupApplicationTest
Feature: dc / acls / tokens / use: Using an ACL token
  Background:
    Given 1 datacenter model with the value "datacenter"
    And 1 token model from yaml
    ---
      AccessorID: token
      SecretID: ee52203d-989f-4f7a-ab5a-2bef004164ca
      Namespace: @!namespace
    ---
    And settings from yaml
    ---
    consul:token:
      SecretID: secret
      AccessorID: accessor
      Namespace: default
      Partition: default
    ---
  Scenario: Using an ACL token from the listing page
    When I visit the tokens page for yaml
    ---
      dc: datacenter
    ---
    And I click actions on the tokens
    And I click use on the tokens
    And I click confirmUse on the tokens
    Then "[data-notification]" has the "notification-use" class
    And "[data-notification]" has the "success" class
    Then I have settings like yaml
    ---
    consul:token: "{\"AccessorID\":\"token\",\"SecretID\":\"ee52203d-989f-4f7a-ab5a-2bef004164ca\",\"Namespace\":\"@namespace\",\"Partition\":\"default\"}"
    ---
  # FIXME
  @ignore
  Scenario: Using an ACL token from the detail page
    When I visit the token page for yaml
    ---
      dc: datacenter
      token: token
    ---
    And I click use
    And I click confirmUse
    Then "[data-notification]" has the "notification-use" class
    And "[data-notification]" has the "success" class
    Then I have settings like yaml
    ---
    consul:token: "{\"AccessorID\":\"token\",\"SecretID\":\"ee52203d-989f-4f7a-ab5a-2bef004164ca\",\"Namespace\":\"@namespace\",\"Partition\":\"default\"}"
    ---
