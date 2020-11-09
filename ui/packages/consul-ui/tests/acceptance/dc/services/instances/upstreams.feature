@setupApplicationTest
Feature: dc / services / instances / Upstreams
  Background:
    Given 1 datacenter model with the value "dc1"
  Scenario: A Service instance without a Proxy does not display Upstreams tab
    Given 1 proxy model from yaml
    ---
    - ServiceProxy:
        DestinationServiceName: service-1
        DestinationServiceID: ~
    ---
    When I visit the instance page for yaml
    ---
      dc: dc1
      service: service-0
      node: node-0
      id: service-0-with-id
    ---
    Then the url should be /dc1/services/service-0/instances/node-0/service-0-with-id/health-checks
    And I don't see upstreams on the tabs
  Scenario: A Service instance with a Proxy displays Upstreams tab
    Given 1 proxy model from yaml
    ---
    - ServiceProxy:
        DestinationServiceName: service-0
        DestinationServiceID: ~
    ---
    When I visit the instance page for yaml
    ---
      dc: dc1
      service: service-0
      node: node-0
      id: service-0-with-id
    ---
    Then the url should be /dc1/services/service-0/instances/node-0/service-0-with-id/health-checks
    And I see upstreams on the tabs

    When I click upstreams on the tabs

    Then the url should be /dc1/services/service-0/instances/node-0/service-0-with-id/upstreams
    And I see upstreamsIsSelected on the tabs

