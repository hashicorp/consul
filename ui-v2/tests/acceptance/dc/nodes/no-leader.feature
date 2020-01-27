@setupApplicationTest
Feature: dc / nodes / no-leader
  Scenario: Leader hasn't been elected
    Given 1 datacenter model with the value "dc-1"
    And 3 node models
    And the url "/v1/status/leader" responds with from yaml
    ---
    body: |
      ""
    ---
    When I visit the nodes page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/nodes
    Then I see 3 node models
    And I don't see leader on the nodes

