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
  Scenario: Viewing nodes list page should not show synthetic nodes
    Given 3 node model from yaml
    ---
    - Meta:
        synthetic-node: true
      Checks:
        - Status: passing
          ServiceID: ""
    - Meta:
        synthetic-node: false
      Checks:
        - Status: passing
          ServiceID: ""
    - Meta:
        synthetic-node: false
      Checks:
        - Status: critical
          ServiceID: ""
    ---
    When I visit the nodes page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/nodes
    Then I see 2 node models
  Scenario: Viewing a node with an unhealthy ServiceCheck
    Given 1 node model from yaml
    ---
    - Checks:
        - Status: passing
          ServiceID: ""
        - Status: critical
          ServiceID: web
      Meta:
        synthetic-node: false
    ---
    When I visit the nodes page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/nodes
    Then I see 1 node models
    And I see status on the nodes.0 like "passing"
  Scenario: Viewing nodes in the listing
    Given 3 node model from yaml
    ---
    - Meta:
        synthetic-node: false
      Checks:
        - Status: passing
          ServiceID: ""
    - Meta:
        synthetic-node: false
      Checks:
        - Status: passing
          ServiceID: ""
    - Meta:
        synthetic-node: false
      Checks:
        - Status: critical
          ServiceID: ""
    ---
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
        Meta:
          synthetic-node: false
      - Address: 10.0.0.1
        Checks:
          - Status: passing
        Meta:
          synthetic-node: false
      - Address: 10.0.0.3
        Checks:
          - Status: passing
        Meta:
          synthetic-node: false
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
        Meta:
          synthetic-node: false
      - Node: node-02
        Address: 10.0.0.1
        Meta:
          synthetic-node: false
      - Node: node-03
        Address: 10.0.0.2
        Meta:
          synthetic-node: false
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
  Scenario: Viewing an empty nodes page with acl enabled
    Given 1 datacenter model with the value "dc-1"
    And 0 nodes models
    When I visit the nodes page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/nodes
    And the title should be "Nodes - Consul"
    Then I see 0 node models
    And I see the text "There don't seem to be any registered Nodes in this Consul cluster, or you may not have service:read and node:read permissions access to this view." in ".empty-state p"
    And I see the "[data-test-empty-state-login]" element

