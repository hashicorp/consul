@setupApplicationTest
Feature: dc / services / show / topology / stats
  Scenario: Given metrics is disabled,the Topology tab should not display metrics
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
    And I don't see the "[data-test-topology-metrics-stats]" element
  Scenario: Given metrics is enabled, the Topology tab should display metrics
    Given 1 datacenter model with the value "dc1"
    Given a "prometheus" metrics provider
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
    And I see the "[data-test-topology-metrics-stats]" element

