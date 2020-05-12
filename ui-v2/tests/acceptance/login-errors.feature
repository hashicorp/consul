@setupApplicationTest
Feature: login-errors: Login Errors

  Scenario: I get any 500 error that is not the specific legacy token cluster one
    Given 1 datacenter model with the value "dc-1"
    Given the url "/v1/acl/tokens?ns=@namespace" responds with a 500 status
    When I visit the tokens page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/tokens
    Then I see status on the error like "500"
  Scenario: I get a 500 error from acl/tokens that is the specific legacy one
    Given 1 datacenter model with the value "dc-1"
    And the url "/v1/acl/tokens?ns=@namespace" responds with from yaml
    ---
    status: 500
    body: "rpc error making call: rpc: can't find method ACL.TokenRead"
    ---
    When I visit the tokens page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/tokens
    Then ".app-view" has the "unauthorized" class
  @notNamespaceable
  Scenario: I get a 500 error from acl/token/self that is the specific legacy one
    Given 1 datacenter model with the value "dc-1"
    Given the url "/v1/acl/tokens?ns=@namespace" responds with from yaml
    ---
    status: 500
    body: "rpc error making call: rpc: can't find method ACL.TokenRead"
    ---
    And the url "/v1/acl/token/self" responds with from yaml
    ---
    status: 500
    body: "rpc error making call: rpc: can't find method ACL.TokenRead"
    ---
    And the url "/v1/acl/list" responds with a 403 status
    When I visit the tokens page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/tokens
    Then ".app-view" has the "unauthorized" class
    And I click login on the navigation
    And I fill in the auth form with yaml
    ---
    SecretID: something
    ---
    And I click submit on the authdialog.form
    Then I see status on the error like "403"

