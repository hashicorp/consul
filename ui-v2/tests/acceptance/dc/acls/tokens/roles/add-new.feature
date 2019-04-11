@setupApplicationTest
Feature: dc / acls / tokens / roles: Add new
  Background:
    Given 1 datacenter model with the value "datacenter"
    And 1 token model from yaml
    ---
      AccessorID: key
      Description: The Description
      Roles: ~
    ---
    And 1 policy model from yaml
    ---
      ID: policy-1
      Name: policy
    ---
    When I visit the token page for yaml
    ---
      dc: datacenter
      token: key
    ---
    Then the url should be /datacenter/acls/tokens/key
    And I click newRole
    Then I fill in the role form with yaml
    ---
      Name: New-Role
      Description: New Role Description
    ---
  Scenario: Add Policy-less Role
    And I click submit on the roleForm
    Then the last PUT request was made to "/v1/acl/role?dc=datacenter" with the body from yaml
    ---
      Name: New-Role
      Description: New Role Description
    ---
    And I submit
    Then a PUT request is made to "/v1/acl/token/key?dc=datacenter" with the body from yaml
    ---
      Description: The Description
      Roles:
        - Name: New-Role
          ID: ee52203d-989f-4f7a-ab5a-2bef004164ca-1
    ---
    Then the url should be /datacenter/acls/tokens
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
  Scenario: Add Role that has an existing Policy
    And I click "#new-role-toggle + div .ember-power-select-trigger"
    And I click ".ember-power-select-option:first-child"
    And I click submit on the roleForm
    Then the last PUT request was made to "/v1/acl/role?dc=datacenter" with the body from yaml
    ---
      Name: New-Role
      Description: New Role Description
      Policies:
        - ID: policy-1
          Name: policy
    ---
    And I submit
    Then a PUT request is made to "/v1/acl/token/key?dc=datacenter" with the body from yaml
    ---
      Description: The Description
      Roles:
        - Name: New-Role
          ID: ee52203d-989f-4f7a-ab5a-2bef004164ca-1
    ---
    Then the url should be /datacenter/acls/tokens
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
  Scenario: Add Role and add a new Policy
    And I click newPolicy on the roleForm
    Then I fill in the policy form on the roleForm component with yaml
    ---
      Name: New-Policy
      Description: New Policy Description
      Rules: key {}
    ---
    # This next line is actually the popped up policyForm due to the way things currently work
    And I click submit on the roleForm
    Then the last PUT request was made to "/v1/acl/policy?dc=datacenter" with the body from yaml
    ---
      Name: New-Policy
      Description: New Policy Description
      Rules: key {}
    ---
    And I click submit on the roleForm
    Then the last PUT request was made to "/v1/acl/role?dc=datacenter" with the body from yaml
    ---
      Name: New-Role
      Description: New Role Description
      Policies:
      # TODO: Ouch, we need to do deep partial comparisons here
        - ID: ee52203d-989f-4f7a-ab5a-2bef004164ca-1
          Name: New-Policy
    ---
    And I submit
    Then a PUT request is made to "/v1/acl/token/key?dc=datacenter" with the body from yaml
    ---
      Description: The Description
      Roles:
        - Name: New-Role
          ID: ee52203d-989f-4f7a-ab5a-2bef004164ca-1
    ---
    Then the url should be /datacenter/acls/tokens
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
@pending:
  Scenario: Click the cancel form
    Then ok
    # And I click cancel on the policyForm
