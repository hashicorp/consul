@setupApplicationTest
Feature: dc / acls / tokens / policies: List
  Scenario:
    Given 1 datacenter model with the value "datacenter"
    And 1 token model from yaml
    ---
      AccessorID: key
      ServiceIdentities:
        - ServiceName: Service-Identity
        - ServiceName: Service-Identity 2
        - ServiceName: Service-Identity 3

      Policies:
        - Name: Policy
          ID: 0000
        - Name: Policy 2
          ID: 0002
        - Name: Policy 3
          ID: 0003
    ---
    When I visit the token page for yaml
    ---
      dc: datacenter
      token: key
    ---
    Then the url should be /datacenter/acls/tokens/key
    # ServiceIdentities turn into policies
    Then I see 6 policy models on the policies component
