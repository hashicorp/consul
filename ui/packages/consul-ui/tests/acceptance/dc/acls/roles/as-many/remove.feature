@setupApplicationTest
Feature: dc / acls / roles / as-many / remove: Remove
  Scenario:
    Given 1 datacenter model with the value "datacenter"
    And 1 token model from yaml
    ---
      AccessorID: key
      Roles:
        - Name: Role
          ID: 00000000-0000-0000-0000-000000000001
    ---
    When I visit the token page for yaml
    ---
      dc: datacenter
      token: key
    ---
    Then the url should be /datacenter/acls/tokens/key
    And I see 1 role model on the roles component
    And I click actions on the roles.selectedOptions
    And I click delete on the roles.selectedOptions
    And I click confirmDelete on the roles.selectedOptions
    And I see 0 role models on the roles component
    And I submit
    Then a PUT request was made to "/v1/acl/token/key?dc=datacenter&ns=@!namespace" from yaml
    ---
      body:
        Roles: []
    ---
    Then the url should be /datacenter/acls/tokens
