@setupApplicationTest
Feature: dc / kvs / index
  Scenario: Viewing kvs in the listing
    Given 1 datacenter model with the value "dc-1"
    And 3 kv models
    When I visit the kvs page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/kv
    And the title should be "Key / Value - Consul"
    Then I see 3 kv models
  Scenario: Viewing kvs with no write access
    Given 1 datacenter model with the value "dc-1"
    And 3 kv models
    And permissions from yaml
    ---
    key:
      write: false
    ---
    When I visit the kvs page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/kv
    And I don't see create

