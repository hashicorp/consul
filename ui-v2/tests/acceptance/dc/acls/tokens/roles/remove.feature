@setupApplicationTest
Feature: dc / acls / tokens / roles: Remove
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
    And I see 1 role model
    And I click actions on the roles
    And I click delete on the roles
    And I click confirmDelete on the roles
    And I see 0 role models
    And I submit
    Then a PUT request is made to "/v1/acl/token/key?dc=datacenter" with the body from yaml
    ---
      Roles: []
    ---
    Then the url should be /datacenter/acls/tokens
