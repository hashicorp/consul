@setupApplicationTest
Feature: dc / services / instances / navigation
  Background:
    Given 1 datacenter model with the value "dc-1"
    And 2 instance models from yaml
    ---
    - Service:
        Service: service-0
        ID: service-a
      Node:
        Node: node-0
    - Service:
        Service: service-0
        ID: service-b
      Node:
        Node: another-node
    ---
  // TODO: Improve mock data to get proper service instance data
  @ignore
  Scenario: Clicking a instance in the listing and back again
    When I visit the service page for yaml
    ---
      dc: dc-1
      service: service-0
    ---
    And I click instances on the tabs
    Then the url should be /dc-1/services/service-0/instances
    Then I see 2 instance models on the instanceList component
    When I click instance on the instanceList.instances component
    Then a GET request was made to "/v1/catalog/connect/service-0?dc=dc-1"
    Then the url should be /dc-1/services/service-0/instances/node-0/service-a/health-checks
    And I click "[data-test-back]"
    Then the url should be /dc1/services/service-0/topology
