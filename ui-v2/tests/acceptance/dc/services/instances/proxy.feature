@setupApplicationTest
Feature: dc / services / instances / proxy: Show Proxy Service Instance
  Scenario: A Proxy Service instance with no exposed checks
    Given 1 datacenter model with the value "dc1"
    And 1 instance model from yaml
    ---
    - Service:
        Kind: connect-proxy
        Name: service-0-proxy
        ID: service-0-proxy-with-id
        Proxy:
          DestinationServiceName: service-0
          Expose:
            Checks: false
            Paths: []
          Upstreams:
            - DestinationType: service
              DestinationName: service-1
              DestinationNamespace: default
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

    When I click serviceChecks on the tabs
    And I don't see exposed on the serviceChecks

    When I click upstreams on the tabs
    And I see upstreamsIsSelected on the tabs
    And I see 2 of the upstreams object
    And I see name on the upstreams like yaml
    ---
    - service-1 default
    - service-group
    ---
    And I see type on the upstreams like yaml
    ---
    - service
    - prepared_query
    ---
    And I don't see exposedPaths on the tabs

  Scenario: A Proxy Service instance with no automatically exposed checks but with paths
    Given 1 datacenter model with the value "dc1"
    And 1 instance model from yaml
    ---
    - Service:
        Kind: connect-proxy
        Name: service-0-proxy
        ID: service-0-proxy-with-id
        Address: 10.0.0.1
        Proxy:
          DestinationServiceName: service-0
          Expose:
            Paths:
              - Path: /grpc-metrics
                Protocol: grpc
                LocalPathPort: 8081
                ListenerPort: 8080
              - Path: /http-metrics
                Protocol: http
                LocalPathPort: 8082
                ListenerPort: 8083
    ---
    When I visit the instance page for yaml
    ---
      dc: dc1
      service: service-0-proxy
      node: node-0
      id: service-0-proxy-with-id
    ---
    Then the url should be /dc1/services/service-0-proxy/node-0/service-0-proxy-with-id
    And I see serviceChecksIsSelected on the tabs

    When I click serviceChecks on the tabs
    And I don't see exposed on the serviceChecks

    When I click exposedPaths on the tabs
    And I see exposedPaths on the tabs
    And I see 2 of the exposedPaths object
    And I see combinedAddress on the exposedPaths like yaml
    ---
    - 10.0.0.1:8080/grpc-metrics
    - 10.0.0.1:8083/http-metrics
    ---
  Scenario: A Proxy Service instance with only automatically exposed checks but no paths
    Given 1 datacenter model with the value "dc1"
    And 1 instance model from yaml
    ---
    - Service:
        Kind: connect-proxy
        Name: service-0-proxy
        ID: service-0-proxy-with-id
        Address: 10.0.0.1
        Proxy:
          DestinationServiceName: service-0
          Expose:
            Checks: true
            Paths: []
      Checks:
        - Name: http-check
          Type: http
    ---
    When I visit the instance page for yaml
    ---
      dc: dc1
      service: service-0-proxy
      node: node-0
      id: service-0-proxy-with-id
    ---
    Then the url should be /dc1/services/service-0-proxy/node-0/service-0-proxy-with-id
    And I see serviceChecksIsSelected on the tabs

    And I don't see exposedPaths on the tabs

    When I click serviceChecks on the tabs
    And I don't see exposed on the serviceChecks

    When I click nodeChecks on the tabs
    And I don't see exposed on the nodeChecks
