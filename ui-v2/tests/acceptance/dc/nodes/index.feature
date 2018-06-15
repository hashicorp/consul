@setupApplicationTest
Feature: Nodes
  Scenario:
    Given 1 datacenter model with the value "dc-1"
    And 3 node models
    When I visit the nodes page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/nodes
    Then I see 3 node models
