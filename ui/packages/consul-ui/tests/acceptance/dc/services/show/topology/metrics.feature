@setupApplicationTest
Feature: dc / services / show / topology / metrics
  Background:
    Given 1 datacenter model with the value "datacenter"
    And the local datacenter is "datacenter"
    And 1 intention model from yaml
    ---
      SourceNS: default
      SourceName: web
      DestinationNS: default
      DestinationName: db
      ID: intention-id
    ---
    And 1 node model
    And 1 service model from yaml
    ---
    - Service:
        Name: web
        Kind: ~
    ---
    And 1 topology model from yaml
    ---
      Downstreams: []
      Upstreams:
        - Name: db
          Namespace: default
          Datacenter: datacenter
          Intention: {}
    ---
  Scenario: Metrics is not enabled with prometheus provider
    When I visit the service page for yaml
    ---
      dc: datacenter
      service: web
    ---
    And I don't see the "[data-test-sparkline]" element
  Scenario: Metrics is enabled with prometheus provider
    Given 1 datacenter model with the value "datacenter"
    And the local datacenter is "datacenter"
    And ui_config from yaml
    ---
    metrics_proxy_enabled: true
    metrics_provider: 'prometheus'
    ---
    When I visit the service page for yaml
    ---
      dc: datacenter
      service: web
    ---
    And I see the "[data-test-sparkline]" element
