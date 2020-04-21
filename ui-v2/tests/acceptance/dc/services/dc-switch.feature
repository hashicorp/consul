@setupApplicationTest
Feature: dc / services / dc-switch : Switching Datacenters
  Scenario: Seeing all services when switching datacenters
    Given 2 datacenter models from yaml
    ---
      - dc-1
      - dc-2
    ---
    And 12 service models from yaml
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
      - Name: Service-3
      - Name: Service-3-proxy
        Kind: 'connect-proxy'
      - Name: Service-4
      - Name: Service-4-proxy
        Kind: 'connect-proxy'
      - Name: Service-5
      - Name: Service-5-proxy
        Kind: 'connect-proxy'
    ---

    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services
    Then I see 6 service models
    When I click dc on the navigation
    And I click dcs.1.name
    Then the url should be /dc-2/services
    Then I see 6 service models
    When I click dc on the navigation
    And I click dcs.0.name
    Then the url should be /dc-1/services
    Then I see 6 service models
    When I click dc on the navigation
    And I click dcs.1.name
    Then the url should be /dc-2/services
    Then I see 6 service models
