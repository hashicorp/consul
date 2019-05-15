@setupApplicationTest
Feature: dc / acls / roles / as many / list: List
  Scenario:
    Given 1 datacenter model with the value "datacenter"
    And 1 token model from yaml
    ---
      AccessorID: key
      Roles:
        - Name: Role
          ID: 0000
        - Name: Role 2
          ID: 0002
        - Name: Role 3
          ID: 0003
    ---
    When I visit the token page for yaml
    ---
      dc: datacenter
      token: key
    ---
    Then the url should be /datacenter/acls/tokens/key
    Then I see 3 role models on the roles component
