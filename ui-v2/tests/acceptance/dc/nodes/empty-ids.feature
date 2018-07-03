@setupApplicationTest
Feature: Hedge for if nodes come in over the API with no ID
  Scenario: A node list with some missing IDs
    Given 1 datacenter model with the value "dc-1"
    And 5 node models from yaml
    ---
    - ID: id-1
      Node: name-1
    - ID: ""
      Node: name-2
    - ID: ""
      Node: name-3
    - ID: ""
      Node: name-4
    - ID: ""
      Node: name-5
    ---
    When I visit the nodes page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/nodes
    Then I see name on the nodes like yaml
    ---
      - name-1
      - name-2
      - name-3
      - name-4
      - name-5

@ignore
  Scenario: Visually comparing
    Then the ".unhealthy" element should look like the "/node_modules/@hashicorp/consul-testing-extras/fixtures/dc/nodes/empty-ids.png" image
