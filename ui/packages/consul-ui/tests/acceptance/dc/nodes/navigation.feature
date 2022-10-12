@setupApplicationTest
Feature: dc / nodes / navigation
  Scenario: Clicking a node in the listing and back again
    Given 1 datacenter model with the value "dc-1"
    And 3 node models from yaml
    ---
      - Node: Node-A
        Meta:
          synthetic-node: "false"
        Checks:
          - Status: critical
            ServiceID: ""
      - Node: Node-B
        Meta:
          synthetic-node: "false"
        Checks:
          - Status: passing
            ServiceID: ""
      - Node: Node-C
        Meta:
          synthetic-node: "false"
        Checks:
          - Status: warning
            ServiceID: ""
    ---
    When I visit the nodes page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/nodes
    And the title should be "Nodes - Consul"
    Then I see 3 node models
    When I click node on the nodes
    And I click "[data-test-back]"
    Then the url should be /dc-1/nodes

