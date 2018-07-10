@setupApplicationTest
Feature: token headers
  In order to authenticate with tokens
  As a user
  I need to be able to specify a ACL token AND/OR leave it blank to authenticate with the API
  Scenario: Arriving at the index page having not set a token previously
    Given 1 datacenter model with the value "datacenter"
    When I visit the index page
    Then the url should be /datacenter/services
    And a GET request is made to "/v1/catalog/datacenters" from yaml
    ---
    headers:
      X-Consul-Token: ''
    ---
  Scenario: Set the token to [Token] and then navigate to the index page
    Given 1 datacenter model with the value "datacenter"
    When I visit the settings page
    Then the url should be /settings
    Then I fill in with yaml
    ---
      token: [Token]
    ---
    And I submit
    When I visit the index page
    Then the url should be /datacenter/services
    And a GET request is made to "/v1/catalog/datacenters" from yaml
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
