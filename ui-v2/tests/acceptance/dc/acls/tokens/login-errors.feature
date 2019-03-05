@setupApplicationTest
Feature: dc / acls / tokens / index: ACL Login Errors

  Scenario: I get any 500 error that is not the specific legacy token cluster one
    Given 1 datacenter model with the value "dc-1"
    Given the url "/v1/acl/tokens" responds with a 500 status
    When I visit the tokens page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/tokens
    Then I see the text "500 (The backend responded with an error)" in "[data-test-error]"
  Scenario: I get a 500 error from acl/tokens that is the specific legacy one
    Given 1 datacenter model with the value "dc-1"
    And the url "/v1/acl/tokens" responds with from yaml
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
  Scenario: I get a 500 error from acl/token/self that is the specific legacy one
    Given 1 datacenter model with the value "dc-1"
    Given the url "/v1/acl/tokens" responds with from yaml
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
    Then I fill in with yaml
    ---
    secret: something
    ---
    And I submit
    Then ".app-view" has the "unauthorized" class
    And "[data-notification]" has the "error" class

