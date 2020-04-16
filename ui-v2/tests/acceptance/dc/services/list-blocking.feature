@setupApplicationTest
Feature: dc / services / list blocking
  Scenario: Viewing the listing pages for service
    Given 1 datacenter model with the value "dc-1"
    And 6 service models from yaml
    ---
      - Name: Service-0
      - Name: Service-0-proxy
        Kind: 'connect-proxy'
      - Name: Service-1
      - Name: Service-1-proxy
        Kind: 'connect-proxy'
      - Name: Service-2
      - Name: Service-2-proxy
        Kind: 'connect-proxy'
    ---
    And a network latency of 100
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services
    And pause until I see 3 service models
    And an external edit results in 5 service models
    And pause until I see 3 service models
    And an external edit results in 1 service model
    And pause until I see 1 service model
    And an external edit results in 0 service models
    And pause until I see 0 service models
