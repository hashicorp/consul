@setupApplicationTest
Feature: dc / services / show / upstreams
  Background:
    Given 1 datacenter model with the value "dc1"
    And 1 node models
    And 1 service model from yaml
    ---
    - Service:
        Name: ingress-gateway-1
        Kind: ingress-gateway
    ---
  Scenario: Seeing the Upstreams tab
    When I visit the service page for yaml
    ---
      dc: dc1
      service: ingress-gateway-1
    ---
    And the title should be "ingress-gateway-1 - Consul"
    And I see upstreams on the tabs
    When I click upstreams on the tabs
    And I see upstreamsIsSelected on the tabs
  Scenario: Seeing the list of Upstreams
    Given 3 service models
    When I visit the service page for yaml
    ---
      dc: dc1
      service: ingress-gateway-1
    ---
    And the title should be "ingress-gateway-1 - Consul"
    When I click upstreams on the tabs
    And I see upstreamsIsSelected on the tabs
    Then I see 3 service models on the tabs.upstreamsTab component
  Scenario: Don't see the Upstreams tab
    Given 1 datacenter model with the value "dc1"
    And 1 node models
    And 1 service model from yaml
    ---
    - Service:
        Name: [Name]
        Kind: [Kind]
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: [Name]
    ---
    And the title should be "[Name] - Consul"
    And I don't see upstreams on the tabs
    Where:
    ---------------------------------------------
    | Name                | Kind                |
    | service             | ~                   |
    | terminating-gateway | terminating-gateway |
    | mesh-gateway        | mesh-gateway        |
    ---------------------------------------------
