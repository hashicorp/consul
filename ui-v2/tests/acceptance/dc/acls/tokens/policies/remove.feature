@setupApplicationTest
Feature: dc / acls / tokens / policies: Remove
  Scenario:
    Given 1 datacenter model with the value "datacenter"
    And 1 token model from yaml
    ---
      AccessorID: key
      Policies:
        - Name: Policy
          ID: 00000000-0000-0000-0000-000000000001
    ---
    When I visit the token page for yaml
    ---
      dc: datacenter
      token: key
    ---
    Then the url should be /datacenter/acls/tokens/key
    And I see 1 policy model
    And I click expand on the policies
    And the last GET request was made to "/v1/acl/policy/00000000-0000-0000-0000-000000000001?dc=datacenter"
    And I click delete on the policies
    And I click confirmDelete on the policies
    And I see 0 policy models
    And I submit
    Then a PUT request is made to "/v1/acl/token/key?dc=datacenter" with the body from yaml
    ---
      Policies: []
    ---
    Then the url should be /datacenter/acls/tokens
