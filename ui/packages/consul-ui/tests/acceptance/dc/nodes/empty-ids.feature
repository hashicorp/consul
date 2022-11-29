@setupApplicationTest
Feature: dc / nodes / empty-ids: Hedge for if nodes come in over the API with no ID
  Scenario: A node list with some missing IDs
    Given 1 datacenter model with the value "dc-1"
    And 5 node models from yaml
    ---
    - ID: id-1
      Node: name-1
      Meta:
        synthetic-node: false
      Checks:
      - Status: passing
    - ID: ""
      Node: name-2
      Meta:
        synthetic-node: false
      Checks:
      - Status: passing
    - ID: ""
      Node: name-3
      Meta:
        synthetic-node: false
      Checks:
      - Status: passing
    - ID: ""
      Node: name-4
      Meta:
        synthetic-node: false
      Checks:
      - Status: passing
    - ID: ""
      Node: name-5
      Meta:
        synthetic-node: false
      Checks:
      - Status: passing
    ---
    When I visit the nodes page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/nodes
    Then I see name on the nodes vertically like yaml
    ---
      - name-1
      - name-2
      - name-3
      - name-4
      - name-5
    ---
