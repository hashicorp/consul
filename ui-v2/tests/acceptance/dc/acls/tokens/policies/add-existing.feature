@setupApplicationTest
Feature: dc / acls / tokens / policies: ACL Token add existing policy
  Scenario:
    Given 1 datacenter model with the value "datacenter"
    And 1 token model from yaml
    ---
      AccessorID: key
      Description: The Description
      Policies: ~
    ---
    And 2 policy models from yaml
    ---
    - ID: policy-1
      Name: Policy 1
    - ID: policy-2
      Name: Policy 2
    ---
    When I visit the token page for yaml
    ---
      dc: datacenter
      token: key
    ---
    Then the url should be /datacenter/acls/tokens/key
    And I click "[data-test-policy-element] .ember-power-select-trigger"
    And I click ".ember-power-select-option:first-child"
    And I see 1 policy model
    And I click "[data-test-policy-element] .ember-power-select-trigger"
    And I click ".ember-power-select-option:nth-child(1)"
    And I see 2 policy models
    Then I fill in with yaml
    ---
      Description: The Description
    ---
    And I submit
    Then a PUT request is made to "/v1/acl/token/key?dc=datacenter" with the body from yaml
    ---
      Description: The Description
      Policies:
      - ID: policy-1
        Name: Policy 1
      - ID: policy-2
        Name: Policy 2
    ---
    Then the url should be /datacenter/acls/tokens
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
