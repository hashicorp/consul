@setupApplicationTest
Feature: dc / services / Show Routing for Serivce
Scenario: Given a service, the Routing tab should display
    Given 1 datacenter model with the value "dc1"
    And 1 node models
    And 1 service model from yaml
    ---
    - Service:
        Kind: consul
        Name: service-0
        ID: service-0-with-id
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service-0
    ---
    And the title should be "service-0 - Consul"
    And I see routing on the tabs
  Scenario: Given a service proxy, the Routing tab should not display
    Given 1 datacenter model with the value "dc1"
    And 1 node models
    And 1 service model from yaml
    ---
    - Service:
        Kind: connect-proxy
        Name: service-0-proxy
        ID: service-0-proxy-with-id
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service-0-proxy
    ---
    And the title should be "service-0-proxy - Consul"
    And I don't see routing on the tabs

