@setupApplicationTest
Feature: dc / services / instances / proxy: Show Proxy Service Instance
  Scenario: A Proxy Service instance
    Given 1 datacenter model with the value "dc1"
    And 1 instance model from yaml
    ---
    - Service:
        Kind: connect-proxy
        Name: service-0-proxy
        ID: service-0-proxy-with-id
        Proxy:
          DestinationServiceName: service-0
          Upstreams:
            - DestinationType: service
              DestinationName: service-1
              LocalBindAddress: 127.0.0.1
              LocalBindPort: 1111
            - DestinationType: prepared_query
              DestinationName: service-group
              LocalBindAddress: 127.0.0.1
              LocalBindPort: 1112
    ---
    When I visit the instance page for yaml
    ---
      dc: dc1
      service: service-0-proxy
      node: node-0
      id: service-0-proxy-with-id
    ---
    Then the url should be /dc1/services/service-0-proxy/node-0/service-0-proxy-with-id
    And I see destination on the proxy like "service"

    And I see serviceChecksIsSelected on the tabs

    When I click upstreams on the tabs
    And I see upstreamsIsSelected on the tabs
    And I see 2 of the upstreams object
    And I see name on the upstreams like yaml
    ---
    - service-1
    - service-group
    ---
    And I see type on the upstreams like yaml
    ---
    - service
    - prepared_query
    ---


