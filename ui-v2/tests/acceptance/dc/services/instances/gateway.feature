@setupApplicationTest
Feature: dc / services / instances / gateway: Show Gateway Service Instance
  Scenario: A Gateway Service instance
    Given 1 datacenter model with the value "dc1"
    Given 1 proxy model from yaml	
    ---	
    - ServiceProxy:	
        DestinationServiceName: service-1	
        DestinationServiceID: ~	
    ---
    And 1 instance model from yaml
    ---
    - Service:
        Kind: mesh-gateway
        Name: gateway
        ID: gateway-with-id
        TaggedAddresses:
          lan:
            Address: 127.0.0.1
            Port: 8080
          wan:
            Address: 92.68.0.0
            Port: 8081
    ---
    When I visit the instance page for yaml
    ---
      dc: dc1
      service: gateway
      node: node-0
      id: gateway-with-id
    ---
    Then the url should be /dc1/services/gateway/instances/node-0/gateway-with-id/health-checks

    And I see healthChecksIsSelected on the tabs

    When I click addresses on the tabs
    And I see addressesIsSelected on the tabs
    And I see 2 of the addresses object
    And I see address on the addresses like yaml
    ---
    - 127.0.0.1:8080
    - 92.68.0.0:8081
    ---
