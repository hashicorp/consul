@setupApplicationTest
Feature: dc / nodes / sorting
  Scenario:
    Given 1 datacenter model with the value "dc-1"
    And 6 node models from yaml
    ---
      - Node: Node-A
        Meta:
          synthetic-node: false
        Checks:
          - Status: critical
            ServiceID: ""
      - Node: Node-B
        Meta:
          synthetic-node: false
        Checks:
          - Status: passing
            ServiceID: ""
      - Node: Node-C
        Meta:
          synthetic-node: false
        Checks:
          - Status: warning
            ServiceID: ""
      - Node: Node-D
        Meta:
          synthetic-node: false
        Checks:
          - Status: critical
            ServiceID: ""
      - Node: Node-E
        Meta:
          synthetic-node: false
        Checks:
          - Status: critical
            ServiceID: ""
      - Node: Node-F
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
    When I click selected on the sort
    When I click options.0.button on the sort
    Then I see name on the nodes vertically like yaml
    ---
    - Node-A
    - Node-D
    - Node-E
    - Node-C
    - Node-F
    - Node-B
    ---
    When I click selected on the sort
    When I click options.1.button on the sort
    Then I see name on the nodes vertically like yaml
    ---
    - Node-B
    - Node-C
    - Node-F
    - Node-A
    - Node-D
    - Node-E
    ---
    When I click selected on the sort
    When I click options.2.button on the sort
    Then I see name on the nodes vertically like yaml
    ---
    - Node-A
    - Node-B
    - Node-C
    - Node-D
    - Node-E
    - Node-F
    ---
    When I click selected on the sort
    When I click options.3.button on the sort
    Then I see name on the nodes vertically like yaml
    ---
    - Node-F
    - Node-E
    - Node-D
    - Node-C
    - Node-B
    - Node-A
    ---
