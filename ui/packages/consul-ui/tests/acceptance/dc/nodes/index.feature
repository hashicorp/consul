@setupApplicationTest
Feature: dc / nodes / index
  Background:
    Given 1 datacenter model with the value "dc-1"
    And the url "/v1/status/leader" responds with from yaml
    ---
    body: |
      "211.245.86.75:8500"
    ---
  Scenario: Viewing a node with an unhealthy NodeCheck
    Given 1 node model from yaml
    ---
    - Checks:
        - Status: critical
          ServiceID: ""
    ---
    When I visit the nodes page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/nodes
    Then I see 1 node models
    And I see status on the nodes.0 like "critical"
  Scenario: Viewing a node with an unhealthy ServiceCheck
    Given 1 node model from yaml
    ---
    - Checks:
        - Status: passing
          ServiceID: ""
        - Status: critical
          ServiceID: web
    ---
    When I visit the nodes page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/nodes
    Then I see 1 node models
    And I see status on the nodes.0 like "passing"
  Scenario: Viewing nodes in the listing
    Given 3 node models
    When I visit the nodes page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/nodes
    And the title should be "Nodes - Consul"
    And a GET request was made to "/v1/internal/ui/nodes?dc=dc-1&ns=@namespace"
    Then I see 3 node models
  Scenario: Seeing the leader in node listing
    Given 3 node models from yaml
    ---
      - Address: 211.245.86.75
        Checks:
          - Status: critical
            Name: Warning check
      - Address: 10.0.0.1
        Checks:
          - Status: passing
      - Address: 10.0.0.3
        Checks:
          - Status: passing
    ---
    When I visit the nodes page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/nodes
    Then I see 3 node models
    And I see leader on the nodes.0
  Scenario: Searching the nodes with name and IP address
    Given 3 node models from yaml
    ---
      - Node: node-01
        Address: 10.0.0.0
      - Node: node-02
        Address: 10.0.0.1
      - Node: node-03
        Address: 10.0.0.2
    ---
    When I visit the nodes page for yaml
    ---
      dc: dc-1
    ---
    And I see 3 node models
    Then I fill in with yaml
    ---
    s: node-01
    ---
    And I see 1 node model
    And I see 1 node model with the name "node-01"
    Then I fill in with yaml
    ---
    s: 10.0.0.1
    ---
    And I see 1 node model
    And I see 1 node model with the name "node-02"
