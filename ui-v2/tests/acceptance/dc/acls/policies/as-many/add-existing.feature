@setupApplicationTest
Feature: dc / acls / policies / as many / add existing: Add existing policy
  Scenario: Adding an existing policy as a child of [Model]
    Given 1 datacenter model with the value "datacenter"
    And 1 [Model] model from yaml
    ---
      Policies: ~
      ServiceIdentities: ~
    ---
    And 2 policy models from yaml
    ---
    - ID: policy-1
      Name: Policy 1
    - ID: policy-2
      Name: Policy 2
    ---
    When I visit the [Model] page for yaml
    ---
      dc: datacenter
      [Model]: key
    ---
    Then the url should be /datacenter/acls/[Model]s/key
    And I click "#policies .ember-power-select-trigger"
    And I click ".ember-power-select-option:first-child"
    And I see 1 policy model on the policies component
    And I click "#policies .ember-power-select-trigger"
    And I click ".ember-power-select-option:nth-child(1)"
    And I see 2 policy models on the policies component
    And I submit
    Then a PUT request is made to "/v1/acl/[Model]/key?dc=datacenter" with the body from yaml
    ---
      Policies:
      - ID: policy-1
        Name: Policy 1
      - ID: policy-2
        Name: Policy 2
    ---
    Then the url should be /datacenter/acls/[Model]s
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
  Where:
    -------------
    | Model     |
    | token     |
    | role      |
    -------------
