@setupApplicationTest
Feature: dc / services / instances / with-proxy: Show Service Instance with a proxy
  Scenario: A Service instance has a Proxy (no DestinationServiceID)
    Given 1 datacenter model with the value "dc1"
    And 1 proxy model from yaml
    ---
    - ServiceProxy:
        DestinationServiceID: ~
    ---
    When I visit the instance page for yaml
    ---
      dc: dc1
      service: service-0
      node: node-0
      id: service-0-with-id
    ---
    Then the url should be /dc1/services/service-0/node-0/service-0-with-id
    And I see type on the proxy like "proxy"

    And I see serviceChecksIsSelected on the tabs
    And I don't see upstreams on the tabs
