@setupApplicationTest
Feature: dc / kvs / create
  Scenario: 
    Given 1 datacenter model with the value "datacenter"
    When I visit the kv page for yaml
    ---
      dc: datacenter
    ---
    Then the url should be /datacenter/kv/create
    And the title should be "New Key/Value - Consul"

@ignore
  Scenario: Test we can create a KV
  Then ok
