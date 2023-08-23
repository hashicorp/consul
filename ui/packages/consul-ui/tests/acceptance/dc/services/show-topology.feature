@setupApplicationTest
Feature: dc / services / show-topology: Show Topology tab for Service
  Scenario: Given a service, the Topology tab should display
    Given 1 datacenter model with the value "dc1"
    And 1 node models
    And 1 service model from yaml
    ---
    - Service:
        Name: service-0
        ID: service-0-with-id
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service-0
    ---
    And I see topology on the tabs
    Then the url should be /dc1/services/service-0/topology
  Scenario: Given connect is disabled, the Topology tab should not display or error
    Given 1 datacenter model with the value "dc1"
    And 1 node models
    And 1 service model from yaml
    ---
    - Service:
        Name: service-0
        ID: service-0-with-id
    ---
    And the url "/v1/discovery-chain/service-0?dc=dc1&ns=@namespace" responds with from yaml
    ---
    status: 500
    body: "Connect must be enabled in order to use this endpoint"
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service-0
    ---
    And I don't see topology on the tabs
    Then the url should be /dc1/services/service-0/instances

