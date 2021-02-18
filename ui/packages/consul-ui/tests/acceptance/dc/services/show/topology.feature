@setupApplicationTest
Feature: dc / services / show / topology: Intention Create
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
  Scenario: Allow a connection between service and upstream by saving an intention
    When I visit the service page for yaml
    ---
      dc: datacenter
      service: web
    ---
    When I click ".consul-topology-metrics [data-test-action]"
    And I click ".consul-topology-metrics [data-test-confirm]"
    And "[data-notification]" has the "success" class
  Scenario: There was an error saving the intention
    Given the url "/v1/connect/intentions/exact?source=default%2Fweb&destination=default%2Fdb&dc=datacenter" responds with a 500 status
    When I visit the service page for yaml
    ---
      dc: datacenter
      service: web
    ---
    When I click ".consul-topology-metrics [data-test-action]"
    And I click ".consul-topology-metrics [data-test-confirm]"
    And "[data-notification]" has the "error" class

