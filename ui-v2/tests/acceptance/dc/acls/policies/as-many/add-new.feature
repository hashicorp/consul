@setupApplicationTest
Feature: dc / acls / policies / as many / add new: Add new policy
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
    Then I fill in the policies.form with yaml
    ---
      Name: New-Policy
      Description: New Policy Description
      Rules: key {}
    ---
    And I click submit on the policies.form
    Then the last PUT request was made to "/v1/acl/policy?dc=datacenter" with the body from yaml
    ---
      Name: New-Policy
      Description: New Policy Description
      Rules: key {}
    ---
    And I submit
    Then a PUT request is made to "/v1/acl/[Model]/key?dc=datacenter" with the body from yaml
    ---
      Policies:
        - Name: New-Policy
          ID: ee52203d-989f-4f7a-ab5a-2bef004164ca-1
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
  Scenario: Adding a new service identity as a child of [Model]
    Then I fill in the policies.form with yaml
    ---
      Name: New-Service-Identity
      Description: New Service Identity Description
    ---
    And I click serviceIdentity on the policies.form
    And I click submit on the policies.form
    And I submit
    Then a PUT request is made to "/v1/acl/[Model]/key?dc=datacenter" with the body from yaml
    ---
      ServiceIdentities:
        - ServiceName: New-Service-Identity
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
@ignore:
  Scenario: Click the cancel form
    Then ok
    # And I click cancel on the policyForm
  Where:
    -------------
    | Model     |
    | token     |
    | role      |
    -------------
@ignore
  Scenario: Make sure the Service Identity Rules are readonly
    Then ok
