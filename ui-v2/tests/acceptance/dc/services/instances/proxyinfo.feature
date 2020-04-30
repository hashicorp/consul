@setupApplicationTest
Feature: dc / services / instances / show: Proxy Info tab
  Background:
    Given 1 datacenter model with the value "dc1"
  Scenario: A Service instance without a Proxy does not display Proxy Info tab
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
    And I don't see proxyInfo on the tabs
  Scenario: A Service instance with a Proxy displays Proxy Info tab
    When I visit the instance page for yaml
    ---
      dc: dc1
      service: service-0
      node: node-0
      id: service-0-with-id
    ---
    Then the url should be /dc1/services/service-0/instances/node-0/service-0-with-id/health-checks
    And I see proxyInfo on the tabs

    When I click proxyInfo on the tabs

    Then the url should be /dc1/services/service-0/instances/node-0/service-0-with-id/proxy
    And I see proxyInfoIsSelected on the tabs
  @notNamespaceable
  Scenario: A Proxy with health checks, upstreams, and exposed paths displays all info
    Given 2 instance models from yaml
    ---
    - Service:
        ID: service-0-with-id
        Kind: consul
      Node:
        Node: node-0
    - Service:
        ID: service-0-with-id-proxy
        Kind: connect-proxy
        Proxy:	
          DestinationServiceName: service-0	
          Expose:	
            Checks: false	
            Paths:
              - Path: /grpc-metrics	
                Protocol: grpc	
                LocalPathPort: 8081	
                ListenerPort: 8080	
              - Path: /http-metrics	
                Protocol: http	
                LocalPathPort: 8082	
                ListenerPort: 8083
              - Path: /http-metrics-2
                Protocol: http	
                LocalPathPort: 8083
                ListenerPort: 8084
          Upstreams:	
            - DestinationType: service	
              DestinationName: service-2
              DestinationNamespace: default	
              LocalBindAddress: 127.0.0.1	
              LocalBindPort: 1111	
            - DestinationType: prepared_query	
              DestinationName: service-3	
              LocalBindAddress: 127.0.0.1	
              LocalBindPort: 1112
      Node:
        Node: node-0
      Checks:
        - Name: Service check
          ServiceID: service-0-proxy
          Output: Output of check
          Status: passing
        - Name: Service check
          ServiceID: service-0-proxy
          Output: Output of check
          Status: warning
        - Name: Service check
          Type: http
          ServiceID: service-0-proxy
          Output: Output of check
          Status: critical
        - Name: Node check
          ServiceID: ""
          Output: Output of check
          Status: passing
        - Name: Node check
          ServiceID: ""
          Output: Output of check
          Status: warning
        - Name: Node check
          ServiceID: ""
          Output: Output of check
          Status: critical
    ---
    When I visit the instance page for yaml
    ---
      dc: dc1
      service: service-0
      node: node-0
      id: service-0-with-id
    ---
    Then the url should be /dc1/services/service-0/instances/node-0/service-0-with-id/health-checks
    And I see proxyInfo on the tabs

    When I click proxyInfo on the tabs
    Then the url should be /dc1/services/service-0/instances/node-0/service-0-with-id/proxy

    And I see 6 of the proxyChecks object

    And I see 2 of the upstreams object
    And I see name on the upstreams like yaml	
    ---	
    - service-2
    - service-3
    ---
  Scenario: A Proxy without health checks does not display Proxy Health section
   And 2 instance models from yaml
    ---
    - Service:
        ID: service-0-with-id
        Kind: consul
      Node:
        Node: node-0
    - Service:
        ID: service-0-with-id-proxy
        Kind: connect-proxy
      Node:
        Node: node-0
      Checks: []
    ---
    When I visit the instance page for yaml
    ---
      dc: dc1
      service: service-0
      node: node-0
      id: service-0-with-id
    ---
    Then the url should be /dc1/services/service-0/instances/node-0/service-0-with-id/health-checks
    And I see proxyInfo on the tabs

    When I click proxyInfo on the tabs
    Then the url should be /dc1/services/service-0/instances/node-0/service-0-with-id/proxy
    And I see 0 of the proxyChecks object
  Scenario: A Proxy without upstreams does not display Upstreams section
    And 2 instance models from yaml
    ---
    - Service:
        ID: service-0-with-id
        Kind: consul
      Node:
        Node: node-0
    - Service:
        ID: service-0-with-id-proxy
        Kind: connect-proxy
        Proxy:	
          Upstreams: []
      Node:
        Node: node-0
    ---
    When I visit the instance page for yaml
    ---
      dc: dc1
      service: service-0
      node: node-0
      id: service-0-with-id
    ---
    Then the url should be /dc1/services/service-0/instances/node-0/service-0-with-id/health-checks
    And I see proxyInfo on the tabs

    When I click proxyInfo on the tabs
    Then the url should be /dc1/services/service-0/instances/node-0/service-0-with-id/proxy
    And I see 0 of the upstreams object
  Scenario: A Proxy without exposed path does not display Exposed Paths section
    And 2 instance models from yaml
    ---
    - Service:
        ID: service-0-with-id
        Kind: consul
      Node:
        Node: node-0
    - Service:
        ID: service-0-with-id-proxy
        Kind: connect-proxy
        Proxy:	
          Expose:	
            Checks: false	
            Paths: []
      Node:
        Node: node-0
    ---
    When I visit the instance page for yaml
    ---
      dc: dc1
      service: service-0
      node: node-0
      id: service-0-with-id
    ---
    Then the url should be /dc1/services/service-0/instances/node-0/service-0-with-id/health-checks
    And I see proxyInfo on the tabs

    When I click proxyInfo on the tabs
    Then the url should be /dc1/services/service-0/instances/node-0/service-0-with-id/proxy
    And I see 0 of the exposedPaths object
    

    
