@setupApplicationTest
Feature: dc / acls / tokens / anonymous no delete: The anonymous token has no delete buttons
  Background:
    Given 1 datacenter model with the value "dc-1"
    And 1 token model from yaml
    ---
      AccessorID: 00000000-0000-0000-0000-000000000002
      Policies: ~
    ---
  Scenario: On the listing page
    When I visit the tokens page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/tokens
    And I click actions on the tokens
    Then I don't see delete on the tokens
  Scenario: On the detail page
    When I visit the token page for yaml
    ---
    dc: dc-1
    token: 00000000-0000-0000-0000-000000000002
    ---
    Then the url should be /dc-1/acls/tokens/00000000-0000-0000-0000-000000000002
    Then I don't see confirmDelete
