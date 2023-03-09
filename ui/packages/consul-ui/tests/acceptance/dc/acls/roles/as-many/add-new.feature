@setupApplicationTest
@notNamespaceable
Feature: dc / acls / roles / as-many / add-new: Add new
  Background:
    Given 1 datacenter model with the value "datacenter"
    And 1 token model from yaml
    ---
      AccessorID: key
      Description: The Description
      Roles: ~
      Policies: ~
      ServiceIdentities: ~
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
    And I click roles.create
    Then I fill in the roles.form with yaml
    ---
      Name: New-Role
      Description: New Role Description
    ---
  Scenario: Add Policy-less Role
    And I click submit on the roles.form
    Then a PUT request was made to "/v1/acl/role?dc=datacenter&ns=@namespace" from yaml
    ---
      body:
        Name: New-Role
        Description: New Role Description
    ---
    And I submit
    Then a PUT request was made to "/v1/acl/token/key?dc=datacenter&ns=@namespace" from yaml
    ---
      body:
        Description: The Description
        Roles:
          - Name: New-Role
            ID: ee52203d-989f-4f7a-ab5a-2bef004164ca-1
    ---
    Then the url should be /datacenter/acls/tokens
    And "[data-notification]" has the "hds-toast" class
    And "[data-notification]" has the "hds-alert--color-success" class
  Scenario: Add Role that has an existing Policy
    And I click "#new-role .ember-power-select-trigger"
    And I click ".ember-power-select-option:first-child"
    And I click submit on the roles.form
    Then a PUT request was made to "/v1/acl/role?dc=datacenter&ns=@namespace" from yaml
    ---
      body:
        Name: New-Role
        Description: New Role Description
        Policies:
          - ID: policy-1
            Name: policy
    ---
    And I submit
    Then a PUT request was made to "/v1/acl/token/key?dc=datacenter&ns=@namespace" from yaml
    ---
      body:
        Description: The Description
        Roles:
          - Name: New-Role
            ID: ee52203d-989f-4f7a-ab5a-2bef004164ca-1
    ---
    Then the url should be /datacenter/acls/tokens
    And "[data-notification]" has the "hds-toast" class
    And "[data-notification]" has the "hds-alert--color-success" class
  Scenario: Add Role and add a new Policy
    And I click roles.form.policies.create
    Then I fill in the roles.form.policies.form with yaml
    ---
      Name: New-Policy
      Description: New Policy Description
      Rules: key {}
    ---
    # This next line is actually the popped up policyForm due to the way things currently work
    And I click submit on the roles.form
    Then a PUT request was made to "/v1/acl/policy?dc=datacenter&ns=@namespace" from yaml
    ---
      body:
        Name: New-Policy
        Description: New Policy Description
        Rules: key {}
    ---
    And I click submit on the roles.form
    Then a PUT request was made to "/v1/acl/role?dc=datacenter&ns=@namespace" from yaml
    ---
      body:
        Name: New-Role
        Description: New Role Description
        Policies:
        # TODO: Ouch, we need to do deep partial comparisons here
          - ID: ee52203d-989f-4f7a-ab5a-2bef004164ca-1
            Name: New-Policy
    ---
    And I submit
    Then a PUT request was made to "/v1/acl/token/key?dc=datacenter&ns=@namespace" from yaml
    ---
      body:
        Description: The Description
        Roles:
          - Name: New-Role
            ID: ee52203d-989f-4f7a-ab5a-2bef004164ca-1
    ---
    Then the url should be /datacenter/acls/tokens
    And "[data-notification]" has the "hds-toast" class
    And "[data-notification]" has the "hds-alert--color-success" class
  Scenario: Add Role and add a new Service Identity
    And I click roles.form.policies.create
    Then I fill in the roles.form.policies.form with yaml
    ---
      Name: New-Service-Identity
    ---
    And I click "[value='service-identity']"
    # This next line is actually the popped up policyForm due to the way things currently work
    And I click submit on the roles.form
    And I click submit on the roles.form
    Then a PUT request was made to "/v1/acl/role?dc=datacenter&ns=@namespace" from yaml
    ---
      body:
        Name: New-Role
        Description: New Role Description
        ServiceIdentities:
          - ServiceName: New-Service-Identity
    ---
    And I submit
    Then a PUT request was made to "/v1/acl/token/key?dc=datacenter&ns=@namespace" from yaml
    ---
      body:
        Description: The Description
        Roles:
          - Name: New-Role
            ID: ee52203d-989f-4f7a-ab5a-2bef004164ca-1
    ---
    Then the url should be /datacenter/acls/tokens
    And "[data-notification]" has the "hds-toast" class
    And "[data-notification]" has the "hds-alert--color-success" class
@ignore:
  Scenario: Click the cancel form
    Then ok
    # And I click cancel on the policyForm
