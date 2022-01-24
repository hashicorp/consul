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
