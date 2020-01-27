@setupApplicationTest
Feature: dc / kvs / edit: KV Viewing
  Scenario:
    Given 1 datacenter model with the value "datacenter"
    And 1 kv model from yaml
    ---
      Key: key
      Session: session-id
    ---
    When I visit the kv page for yaml
    ---
      dc: datacenter
      kv: key
    ---
    Then the url should be /datacenter/kv/key/edit
    And I see ID on the session like "session-id"
    Given 1 kv model from yaml
    ---
      Key: another-key
      Session: ~
    ---
    When I visit the kv page for yaml
    ---
      dc: datacenter
      kv: another-key
    ---
    Then I don't see ID on the session
