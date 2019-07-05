@setupApplicationTest
Feature: dc / services / instances / sidecar-proxy: Show Sidecar Proxy Service Instance
  Scenario: A Sidecar Proxy Service instance
    Given 1 datacenter model with the value "dc1"
    And 1 service model from yaml
    ---
    - Service:
        Kind: connect-proxy
        Name: service-0-sidecar-proxy
        ID: service-0-sidecar-proxy-with-id
        Proxy:
          DestinationServiceName: service-0
          DestinationServiceID: service-0-with-id
    ---
    When I visit the instance page for yaml
    ---
      dc: dc1
      service: service-0-sidecar-proxy
      node: node-0
      id: service-0-sidecar-proxy-with-id
    ---
    Then the url should be /dc1/services/service-0-sidecar-proxy/node-0/service-0-sidecar-proxy-with-id
    And I see destination on the proxy like "instance"

    And I see serviceChecksIsSelected on the tabs

    When I click upstreams on the tabs
    And I see upstreamsIsSelected on the tabs


