@setupApplicationTest
Feature: dc / acls / tokens / own no delete: The your current token has no delete buttons
  Background:
    Given 1 datacenter model with the value "dc-1"
    And 1 token model from yaml
    ---
      AccessorID: token
      SecretID: ee52203d-989f-4f7a-ab5a-2bef004164ca
    ---
  Scenario: On the listing page
    Then I have settings like yaml
    ---
    consul:token: ~
    ---
    When I visit the tokens page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/tokens
    And I click actions on the tokens
    And I click use on the tokens
    And I click confirmUse on the tokens
    Then "[data-notification]" has the "notification-use" class
    And "[data-notification]" has the "success" class
    Then I have settings like yaml
    ---
    consul:token: "{\"AccessorID\":\"token\",\"SecretID\":\"ee52203d-989f-4f7a-ab5a-2bef004164ca\"}"
    ---
    And I click actions on the tokens
    Then I don't see delete on the tokens
    And I visit the token page for yaml
    ---
    dc: dc-1
    token: ee52203d-989f-4f7a-ab5a-2bef004164ca
    ---
    Then the url should be /dc-1/acls/tokens/ee52203d-989f-4f7a-ab5a-2bef004164ca
    Then I don't see confirmDelete
