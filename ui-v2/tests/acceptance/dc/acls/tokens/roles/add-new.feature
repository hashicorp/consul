@setupApplicationTest
Feature: dc / acls / tokens / roles: Add new
  Scenario:
    Given 1 datacenter model with the value "datacenter"
    And 1 token model from yaml
    ---
      AccessorID: key
      Description: The Description
      Roles: ~
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
@pending:
  Scenario: Click the cancel form
    Then ok
    # And I click cancel on the policyForm
