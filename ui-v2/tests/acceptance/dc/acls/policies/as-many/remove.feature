@setupApplicationTest
Feature: dc / acls / policies / as many / remove: Remove
  Scenario: Removing policies as children of the [Model] page
    Given 1 datacenter model with the value "datacenter"
    And 1 [Model] model from yaml
    ---
      ServiceIdentities: ~
      Policies:
        - Name: Policy
          ID: 00000000-0000-0000-0000-000000000001
    ---
    When I visit the [Model] page for yaml
    ---
      dc: datacenter
      [Model]: key
    ---
    Then the url should be /datacenter/acls/[Model]s/key
    And I see 1 policy model on the policies component
    And I click expand on the policies.selectedOptions
    And the last GET request was made to "/v1/acl/policy/00000000-0000-0000-0000-000000000001?dc=datacenter"
    And I click delete on the policies.selectedOptions
    And I click confirmDelete on the policies.selectedOptions
    And I see 0 policy models on the policies component
    And I submit
    Then a PUT request is made to "/v1/acl/[Model]/key?dc=datacenter" with the body from yaml
    ---
      Policies: [[]]
    ---
    Then the url should be /datacenter/acls/[Model]s
  Where:
    -------------
    | Model     |
    | token     |
    | role      |
    -------------
