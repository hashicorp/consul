@setupApplicationTest
Feature: dc / services / instances / gateway: Show Gateway Service Instance
  Scenario: A Gateway Service instance
    Given 1 datacenter model with the value "dc1"
    And 1 instance model from yaml
    ---
    - Service:
        Name: gateway
        ID: gateway-with-id
        TaggedAddresses:
          lan:
            Address: 127.0.0.1
            Port: 8080
          wan:
            Address: 92.68.0.0
            Port: 8081
        Proxy:
          MeshGateway: {}
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
      service: gateway
      node: node-0
      id: gateway-with-id
    ---
    Then the url should be /dc1/services/gateway/node-0/gateway-with-id

    And I see serviceChecksIsSelected on the tabs

    When I click addresses on the tabs
    And I see addressesIsSelected on the tabs
    And I see 2 of the addresses object
    And I see address on the addresses like yaml
    ---
    - 127.0.0.1:8080
    - 92.68.0.0:8081
    ---

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


