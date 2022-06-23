@setupApplicationTest
@notNamespaceable
Feature: token-header
  In order to authenticate with tokens
  As a user
  I need to be able to specify a ACL token AND/OR leave it blank to authenticate with the API
  Scenario: Arriving at the service page having not set a token previously
    Given 1 datacenter model with the value "dc1"
    When I visit the services page for yaml
    ---
      dc: dc1
    ---
    Then the url should be /dc1/services
    And a GET request was made to "/v1/internal/ui/services?dc=dc1&with-peers=true&ns=@namespace" from yaml
    ---
    headers:
      X-Consul-Token: ''
    ---
  Scenario: Set the token to [Token] and then navigate to the service page
    Given 1 datacenter model with the value "dc1"
    And the url "/v1/acl/tokens" responds with a 403 status
    When I visit the tokens page for yaml
    ---
      dc: dc1
    ---
    Then the url should be /dc1/acls/tokens
    And I click login on the navigation
    And I fill in the auth form with yaml
    ---
    SecretID: [Token]
    ---
    And I click submit on the authdialog.form
    When I visit the services page for yaml
    ---
      dc: dc1
    ---
    Then the url should be /dc1/services
    And a GET request was made to "/v1/internal/ui/services?dc=dc1&with-peers=true&ns=@namespace" from yaml
    ---
    headers:
      X-Consul-Token: [Token]
    ---
  Where:
      ---------
      | Token |
      | token |
      | ''    |
      ---------
