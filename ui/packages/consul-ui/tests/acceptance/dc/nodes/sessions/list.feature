@setupApplicationTest
Feature: dc / nodes / sessions / list: List Lock Sessions
  In order to get information regarding lock sessions
  As a user
  I should be able to see a listing of lock sessions with necessary information under the lock sessions tab for a node
  Scenario: Given 2 session with string TTLs
    Given 1 datacenter model with the value "dc1"
    And 1 node model from yaml
    ---
    ID: node-0
    ---
    And 2 session models from yaml
    ---
    - TTL: 30s
    - TTL: 60m
    ---
    When I visit the node page for yaml
    ---
      dc: dc1
      node: node-0
    ---
    And I click lockSessions on the tabs
    Then I see lockSessionsIsSelected on the tabs
    Then I see TTL on the sessions like yaml
    ---
    - 30s
    - 60m
    ---
  Scenario: Given 3 session with LockDelay in nanoseconds
    Given 1 datacenter model with the value "dc1"
    And 1 node model from yaml
    ---
    ID: node-0
    ---
    And 3 session models from yaml
    ---
    - LockDelay: 120000
    - LockDelay: 18000000
    - LockDelay: 15000000000
    ---
    When I visit the node page for yaml
    ---
      dc: dc1
      node: node-0
    ---
    And I click lockSessions on the tabs
    Then I see lockSessionsIsSelected on the tabs
    Then I see delay on the sessions like yaml
    ---
    - 120µs
    - 18ms
    - 15s
    ---
  Scenario: Given 0 sessions with ACLs enabled 
    Given 1 datacenter model with the value "dc1"
    And 1 node model from yaml
    ---
    ID: node-0
    ---
    And 0 session models
    When I visit the node page for yaml
    ---
      dc: dc1
      node: node-0
    ---
    And I click lockSessions on the tabs
    Then I see lockSessionsIsSelected on the tabs
    And I see the text "Consul provides a session mechanism which can be used to build distributed locks. Sessions act as a binding layer between Nodes, Health Checks, and Key/Value data. There are currently no Lock Sessions present, or you may not have key:read or session:read permissions." in ".empty-state p"
    And I see the "[data-test-empty-state-login]" element
  Scenario: Given 0 sessions with ACLs disabled 
    Given ACLs are disabled
    Given 1 datacenter model with the value "dc1"
    And 1 node model from yaml
    ---
    ID: node-0
    ---
    And 0 session models
    When I visit the node page for yaml
    ---
      dc: dc1
      node: node-0
    ---
    And I click lockSessions on the tabs
    Then I see lockSessionsIsSelected on the tabs
    And I see the text "Consul provides a session mechanism which can be used to build distributed locks. Sessions act as a binding layer between Nodes, Health Checks, and Key/Value data. There are currently no Lock Sessions present." in ".empty-state p"
    And I don't see the "[data-test-empty-state-login]" element
