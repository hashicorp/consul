@setupApplicationTest
Feature: dc / kvs / edit: KV Viewing
  Scenario: Viewing a KV with a URL unsafe character
    Given 1 datacenter model with the value "datacenter"
    And 1 kv model from yaml
    ---
      Key: "@key"
    ---
    When I visit the kv page for yaml
    ---
      dc: datacenter
      kv: "@key"
    ---
    Then the url should be /datacenter/kv/%40key/edit
    And I see Key on the kv like "@key"
  Scenario: Viewing a Session attached to a KV
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
  Scenario: Viewing a Session attached to a KV
    Given 1 datacenter model with the value "datacenter"
    And 1 kv model from yaml
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
  Scenario: Viewing a kv with no write access
    Given 1 datacenter model with the value "datacenter"
    And 1 kv model from yaml
    ---
      Key: key
      Session: session-id
    ---
    And permissions from yaml
    ---
    key:
      write: false
    session:
      read: false
    ---
    When I visit the kv page for yaml
    ---
      dc: datacenter
      kv: key
    ---
    Then the url should be /datacenter/kv/key/edit
    And I don't see create
    And I don't see ID on the session
    And I see warning on the session
  Scenario: Viewing a kv with no read access
    Given 1 datacenter model with the value "datacenter"
    And 1 kv model from yaml
    ---
      Key: key
    ---
    And permissions from yaml
    ---
    key:
      write: false
      read: false
    ---
    When I visit the kv page for yaml
    ---
      dc: datacenter
      kv: key
    ---
    Then the url should be /datacenter/kv/key/edit
    And I see status on the error like "403"
    And a GET request wasn't made to "/v1/kv/key?dc=datacenter"
  # Make sure we can view KVs that have similar names to sections in the UI
  Scenario: I have KV called [Page]
    Given 1 datacenter model with the value "datacenter"
    And 1 kv model from yaml
    ---
      Key: [Page]
    ---
    When I visit the kv page for yaml
    ---
      dc: datacenter
      kv: [Page]
    ---
    Then the url should be /datacenter/kv/[Page]/edit
  Where:
    --------------
    | Page       |
    | services   |
    | nodes      |
    | intentions |
    | kvs        |
    --------------
