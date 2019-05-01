@setupApplicationTest
Feature: dc / acls / roles / as many / add existing: Add existing
  Scenario:
    Given 1 datacenter model with the value "datacenter"
    And 1 token model from yaml
    ---
      AccessorID: key
      Description: The Description
      Roles: ~
    ---
    And 2 role models from yaml
    ---
    - ID: role-1
      Name: Role 1
    - ID: role-2
      Name: Role 2
    ---
    When I visit the token page for yaml
    ---
      dc: datacenter
      token: key
    ---
    Then the url should be /datacenter/acls/tokens/key
    And I click "#roles .ember-power-select-trigger"
    And I click ".ember-power-select-option:first-child"
    And I see 1 role model on the roles component
    And I click "#roles .ember-power-select-trigger"
    And I click ".ember-power-select-option:nth-child(1)"
    And I see 2 role models on the roles component
    Then I fill in with yaml
    ---
      Description: The Description
    ---
    And I submit
    Then a PUT request is made to "/v1/acl/token/key?dc=datacenter" with the body from yaml
    ---
      Description: The Description
      Roles:
      - ID: role-1
        Name: Role 1
      - ID: role-2
        Name: Role 2
    ---
    Then the url should be /datacenter/acls/tokens
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
