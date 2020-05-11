@setupApplicationTest
Feature: dc / services / gateway
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
    And I see linkedServices on the tabs
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
    Then I see 3 service models

    

