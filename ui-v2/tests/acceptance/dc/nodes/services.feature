@setupApplicationTest
Feature: Search services within nodes by name and port
  Scenario: Given 1 node
    Given 1 datacenter model with the value "dc1"
    And 1 node models from yaml
    ---
    - ID: node-0
    ---
    When I visit the node page for yaml
    ---
      dc: dc1
      node: node-0
    ---
    When I click services on the tabs
    And I see servicesIsSelected on the tabs

