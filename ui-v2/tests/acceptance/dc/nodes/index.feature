@setupApplicationTest
Feature: dc / nodes / index
  Background:
    Given 1 datacenter model with the value "dc-1"
    And the url "/v1/status/leader" responds with from yaml
    ---
    body: |
      "211.245.86.75:8500"
    ---
  Scenario: Viewing nodes in the listing
    Given 3 node models
    When I visit the nodes page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/nodes
    Then I see 3 node models
  Scenario: Seeing the leader in unhealthy listing
    Given 3 node models from yaml
    ---
      - Address: 211.245.86.75
        Checks:
          - Status: warning
            Name: Warning check
      - Address: 10.0.0.1
      - Address: 10.0.0.3
    ---
    When I visit the nodes page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/nodes
    Then I see 3 node models
    And I see leader on the unHealthyNodes
  Scenario: Seeing the leader in healthy listing
    Given 3 node models from yaml
    ---
      - Address: 211.245.86.75
        Checks:
          - Status: passing
            Name: Passing check
      - Address: 10.0.0.1
      - Address: 10.0.0.3
    ---
    When I visit the nodes page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/nodes
    Then I see 3 node models
    And I see leader on the healthyNodes
