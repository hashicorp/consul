@setupApplicationTest
Feature: dc / acls / tokens / clone: Cloning an ACL token
  Background:
    Given 1 datacenter model with the value "datacenter"
    And 1 token model from yaml
    ---
      AccessorID: token
      SecretID: ee52203d-989f-4f7a-ab5a-2bef004164ca
      Legacy: ~
    ---
  Scenario: Cloning an ACL token from the listing page
    When I visit the tokens page for yaml
    ---
      dc: datacenter
    ---
    And I click actions on the tokens
    And I click clone on the tokens
    Then a PUT request was made to "/v1/acl/token/token/clone?dc=datacenter&ns=@!namespace"
    Then "[data-notification]" has the "notification-clone" class
    And "[data-notification]" has the "success" class
  Scenario: Using an ACL token from the detail page
    When I visit the token page for yaml
    ---
      dc: datacenter
      token: token
    ---
    And I click clone
    Then the url should be /datacenter/acls/tokens
    Then "[data-notification]" has the "notification-clone" class
    And "[data-notification]" has the "success" class
