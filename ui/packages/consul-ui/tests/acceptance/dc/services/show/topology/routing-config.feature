@setupApplicationTest
Feature: dc / services / show / topology / routing-config
  Background:
    Given 1 datacenter model with the value "dc1"
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
          Source: routing-config
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service-0
    ---
    And I see topology on the tabs
  Scenario: Given the Source is routing config, show Source Type
    Then I see the text "Routing configuration" in "[data-test-topology-metrics-source-type]"
  Scenario: Given the Source is routing config, redirect to Routing Config page
    When I click "[data-test-topology-metrics-card]"
    Then the url should be /dc1/routing-config/db


