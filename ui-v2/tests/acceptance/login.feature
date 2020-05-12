@setupApplicationTest
Feature: login
  Scenario: Logging into the login page from ACLs tokens
    Given 1 datacenter model with the value "dc-1"
    And the url "/v1/acl/tokens" responds with a 403 status
    When I visit the tokens page for yaml
    ---
    dc: dc-1
    ---
    Then the url should be /dc-1/acls/tokens
    And I click login on the navigation
    And I fill in the auth form with yaml
    ---
    SecretID: something
    ---
    And I click submit on the authdialog.form
    Then a GET request was made to "/v1/acl/token/self?dc=dc-1" from yaml
    ---
    headers:
      X-Consul-Token: something
    ---
