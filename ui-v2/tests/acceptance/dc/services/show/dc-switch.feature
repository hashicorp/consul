@setupApplicationTest
Feature: dc / services / show / dc-switch : Switching Datacenters
  Scenario: Seeing all services when switching datacenters
    Given 2 datacenter models from yaml
    ---
      - dc-1
      - dc-2
    ---
    And 1 node model
    And 1 service model from yaml
    ---
    - Service:
        Service: consul
        Kind: ~
    ---

    When I visit the service page for yaml
    ---
      dc: dc-1
      service: consul
    ---

    Then the url should be /dc-1/services/consul/topology
    When I click dc on the navigation
    And I click dcs.1.name on the navigation
    Then the url should be /dc-2/services/consul/topology
