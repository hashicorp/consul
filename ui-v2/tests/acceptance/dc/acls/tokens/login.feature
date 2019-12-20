@setupApplicationTest
Feature: dc / acls / tokens / login
  Scenario: Logging into the ACLs login page
    Given 1 datacenter model with the value "dc-1"
    And the url "/v1/acl/tokens" responds with a 403 status
    When I visit the tokens page for yaml
    ---
    dc: dc-1
    ---
    Then the url should be /dc-1/acls/tokens
    And I fill in with yaml
    ---
    secret: something
    ---
    And I submit
    Then a GET request was made to "/v1/acl/token/self?dc=dc-1" from yaml
    ---
    headers:
      X-Consul-Token: something
    ---
