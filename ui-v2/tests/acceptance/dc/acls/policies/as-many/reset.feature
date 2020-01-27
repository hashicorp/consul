@setupApplicationTest
Feature: dc / acls / policies / as many / reset: Reset policy form
  Background:
    Given 1 datacenter model with the value "datacenter"
    And 1 [Model] model from yaml
    ---
      Policies: ~
      ServiceIdentities: ~
    ---
    When I visit the [Model] page for yaml
    ---
      dc: datacenter
      [Model]: key
    ---
    Then the url should be /datacenter/acls/[Model]s/key
    And I click policies.create
  Scenario: Adding a new policy as a child of [Model]
    Then I fill in the policies.form form with yaml
    ---
      Name: New-Policy
      Description: New Policy Description
      Rules: operator {}
    ---
    And I click cancel on the policies.form
    And I click policies.create
    Then I see the policies.form form with yaml
    ---
      Name: ""
      Description: ""
      Rules: ""
    ---
  Where:
    -------------
    | Model     |
    | token     |
    | role      |
    -------------
