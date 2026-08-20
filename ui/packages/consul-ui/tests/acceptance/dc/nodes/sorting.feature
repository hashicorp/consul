@setupApplicationTest
Feature: dc / nodes / sorting
  Scenario: Sorting the node list by name
    Given 1 datacenter model with the value "dc-1"
    And 4 node models from yaml
    ---
      - Node: Node-B
        Meta:
          synthetic-node: false
      - Node: Node-D
        Meta:
          synthetic-node: false
      - Node: Node-A
        Meta:
          synthetic-node: false
      - Node: Node-C
        Meta:
          synthetic-node: false
    ---
    When I visit the nodes page for yaml
    ---
      dc: dc-1
    ---
    # ascending (A-Z)
    When I click name on the sort
    Then I see name on the nodes vertically like yaml
    ---
    - Node-A
    - Node-B
    - Node-C
    - Node-D
    ---
    # descending (Z-A)
    When I click name on the sort
    Then I see name on the nodes vertically like yaml
    ---
    - Node-D
    - Node-C
    - Node-B
    - Node-A
    ---
  Scenario: Sorting the node list by health
    Given 1 datacenter model with the value "dc-1"
    And 3 node models from yaml
    ---
      - Node: Node-passing
        Meta:
          synthetic-node: false
        Checks:
          - Status: passing
            ServiceID: ""
      - Node: Node-critical
        Meta:
          synthetic-node: false
        Checks:
          - Status: critical
            ServiceID: ""
      - Node: Node-warning
        Meta:
          synthetic-node: false
        Checks:
          - Status: warning
            ServiceID: ""
    ---
    When I visit the nodes page for yaml
    ---
      dc: dc-1
    ---
    # ascending (unhealthy first)
    When I click health on the sort
    Then I see name on the nodes vertically like yaml
    ---
    - Node-critical
    - Node-warning
    - Node-passing
    ---
    # descending (healthy first)
    When I click health on the sort
    Then I see name on the nodes vertically like yaml
    ---
    - Node-passing
    - Node-warning
    - Node-critical
    ---
