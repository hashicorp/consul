@setupApplicationTest
Feature: dc / services / instances / with-sidecar: Show Service Instance with a Sidecar Proxy
  Scenario: A Service instance has a Sidecar Proxy (a DestinationServiceID)
    Given 1 datacenter model with the value "dc1"
    And 1 proxy model from yaml
    ---
    - ServiceProxy:
        DestinationServiceID: service-1
    ---
    When I visit the instance page for yaml
    ---
      dc: dc1
      service: service-0
      id: service-0-with-id
    ---
    Then the url should be /dc1/services/service-0/service-0-with-id
    And I see type on the proxy like "sidecar-proxy"

    And I see serviceChecksIsSelected on the tabs
    And I don't see upstreams on the tabs


