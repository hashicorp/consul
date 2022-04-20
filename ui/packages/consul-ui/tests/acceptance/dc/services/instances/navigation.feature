@setupApplicationTest
Feature: dc / services / instances / navigation
  Background:
    Given 1 datacenter model with the value "dc-1"
    And 1 proxy model from yaml
    ---
    ServiceName: service-0-proxy
    Node: node-0
    ServiceID: service-a-proxy
    ---
    And 3 instance models from yaml
    ---
    - Service:
        Service: service-0
        ID: service-a
      Node:
        Node: node-0
      Checks:
      - Status: critical
    - Service:
        Service: service-0
        ID: service-b
      Node:
        Node: node-0
      Checks:
      - Status: passing
    # A listing of instances from 2 services would never happen in consul but
    # this satisfies our mocking needs for the moment, until we have a 'And 1
    # proxy on request.0 from yaml', 'And 1 proxy on request.1 from yaml' or
    # similar
    - Service:
        Service: service-0-proxy
        ID: service-a-proxy
      Node:
        Node: node-0
      Checks:
      - Status: passing
    ---
  Scenario: Clicking a instance in the listing and back again
    When I visit the service page for yaml
    ---
      dc: dc-1
      service: service-0
    ---
    And I click instances on the tabs
    Then the url should be /dc-1/services/service-0/instances
    Then I see 3 instance models
    When I click instance on the instances component
    Then a GET request was made to "/v1/catalog/connect/service-0?dc=dc-1&ns=@namespace"
    Then a GET request was made to "/v1/health/service/service-0-proxy?dc=dc-1&ns=@namespace"
    Then the url should be /dc-1/services/service-0/instances/node-0/service-a/health-checks
    And I click "[data-test-back]"
    Then the url should be /dc-1/services/service-0/topology
    And I click instances on the tabs
    When I click instance on the instances component
    Then a GET request was made to "/v1/catalog/connect/service-0?dc=dc-1&ns=@namespace"
    Then a GET request was made to "/v1/health/service/service-0-proxy?dc=dc-1&ns=@namespace"
    Then the url should be /dc-1/services/service-0/instances/node-0/service-a/health-checks
