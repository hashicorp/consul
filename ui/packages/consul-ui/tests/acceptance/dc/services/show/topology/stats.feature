@setupApplicationTest
Feature: dc / services / show / topology / stats
  Scenario: Given metrics is disabled, the Topology tab should not display metrics
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
  Scenario: Given metrics is enabled, metrics stats are disabled for an ingress gateway Topology
    Given 1 datacenter model with the value "dc1"
    Given a "prometheus" metrics provider
    And 1 node models
    And 1 service model from yaml
    ---
    - Service:
        Name: ingress-gateway
        Kind: ingress-gateway
        ID: ingress-gateway-with-id
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: ingress-gateway
    ---
    And I see topology on the tabs
    And I don't see the "[data-test-topology-metrics-stats]" element
    And I see the "[data-test-topology-metrics-status]" element
  Scenario: Given metrics is enabled, metric stats are disabled for ingress gateway as downstream services
    Given 1 datacenter model with the value "dc1"
    Given a "prometheus" metrics provider
    And 1 node models
    And 1 service model from yaml
    ---
    - Service:
        Name: service-0
        ID: service-0-with-id
    ---
    And 1 topology model from yaml
    ---
      Upstreams: []
      Downstreams:
        - Name: db
          Namespace: @namespace
          Datacenter: dc1
          Intention: {}
          Kind: ingress-gateway
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service-0
    ---
    And I see topology on the tabs
    And I see the "[data-test-sparkline]" element
    And I don't see the "[data-test-topology-metrics-downstream-stats]" element

