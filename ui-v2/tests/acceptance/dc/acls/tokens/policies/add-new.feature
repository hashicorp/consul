@setupApplicationTest
Feature: dc / acls / tokens / policies: Add new
  Scenario:
    Given 1 datacenter model with the value "datacenter"
    And 1 token model from yaml
    ---
      AccessorID: key
      Description: The Description
      Policies: ~
    ---
    When I visit the token page for yaml
    ---
      dc: datacenter
      token: key
    ---
    Then the url should be /datacenter/acls/tokens/key
    And I click newPolicy
    Then I fill in the policy form with yaml
    ---
      Name: New-Policy
      Description: New Description
      Rules: key {}
    ---
    And I click submit on the policyForm
    Then the last PUT request was made to "/v1/acl/policy?dc=datacenter" with the body from yaml
    ---
      Name: New-Policy
      Description: New Description
      Rules: key {}
    ---
    And I submit
    Then a PUT request is made to "/v1/acl/token/key?dc=datacenter" with the body from yaml
    ---
      Description: The Description
      Policies:
        - Name: New-Policy
          ID: ee52203d-989f-4f7a-ab5a-2bef004164ca-1
    ---
    Then the url should be /datacenter/acls/tokens
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
@pending:
  Scenario: Click the cancel form
    Then ok
    # And I click cancel on the policyForm
