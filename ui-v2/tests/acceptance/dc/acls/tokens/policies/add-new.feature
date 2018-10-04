@setupApplicationTest
Feature: dc / acls / tokens / policies: Add new
  Scenario:
    Given 1 datacenter model with the value "datacenter"
    And 1 token model from yaml
    ---
      AccessorID: key
      Policies: ~
    ---
    When I visit the token page for yaml
    ---
      dc: datacenter
      token: key
    ---
    Then the url should be /datacenter/acls/tokens/key
    And I click newPolicy
    Then I fill in the policy with yaml
    ---
      Name: New Policy
      Description: New Description
      # Rules: [Rules]
    ---
    And I click submit on the policyForm
    Then the last PUT request was made to "/v1/acl/policy?dc=datacenter" with the body from yaml
    ---
      Name: New Policy
      Description: New Description
      # Rules: [Rules]
    ---
    # And I click cancel on the policyForm
