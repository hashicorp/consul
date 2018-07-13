@setupApplicationTest
Feature: dc / kvs / trailing slash
  Scenario: I have 10 folders
    Given 1 datacenter model with the value "datacenter"
    And 10 kv models from yaml
    When I visit the kvs page for yaml
    ---
      dc: datacenter
      kv: foo/bar
    ---
    Then the url should be /datacenter/kv/foo/bar/
    And the last GET request was made to "/v1/kv/foo/bar/?keys&dc=datacenter&separator=%2F"
    When I visit the kvs page for yaml
    ---
      dc: datacenter
      kv: foo/bar/
    ---
    Then the url should be /datacenter/kv/foo/bar/
    And the last GET request was made to "/v1/kv/foo/bar/?keys&dc=datacenter&separator=%2F"
