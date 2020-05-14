@setupApplicationTest
Feature: dc / services / intentions-error: An error with intentions doesn't 500 the page
  Scenario:
    Given 1 datacenter model with the value "dc1"
    And 1 node model
    And 1 service model from yaml
    ---
    - Service:
        Kind: ~
        Name: service-0
        ID: service-0-with-id
    ---
    And the url "/v1/connect/intentions" responds with a 500 status
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service-0
    ---
    And the title should be "service-0 - Consul"
    And I see 1 instance model
