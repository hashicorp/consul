@setupApplicationTest
Feature: dc / services / show / services
  Background:
    Given 1 datacenter model with the value "dc1"
    And 1 node models
    And 1 service model from yaml
    ---
    - Service:
        Name: terminating-gateway-1
        Kind: terminating-gateway
    ---
  Scenario: Seeing the Linked Services tab
    When I visit the service page for yaml
    ---
      dc: dc1
      service: terminating-gateway-1
    ---
    And the title should be "terminating-gateway-1 - Consul"
    And I see linkedServicesIsVisible on the tabs
    When I click linkedServices on the tabs
    And I see linkedServicesIsSelected on the tabs
  Scenario: Seeing the list of Linked Services
    Given 3 service models from yaml
    When I visit the service page for yaml
    ---
      dc: dc1
      service: terminating-gateway-1
    ---
    And the title should be "terminating-gateway-1 - Consul"
    When I click linkedServices on the tabs
    Then I see 3 service models on the tabs.linkedServicesTab component
  Scenario: Don't see the Linked Services tab
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
    And I don't see linkedServices on the tabs
    Where:
    ---------------------------------------------
    | Name                | Kind                |
    | service             | ~                   |
    | ingress-gateway     | ingress-gateway     |
    | api-gateway         | api-gateway         |
    | mesh-gateway        | mesh-gateway        |
    ---------------------------------------------



