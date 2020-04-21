@setupApplicationTest
Feature: dc / services / list
  Scenario: Listing service
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
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services

    Then I see 3 service models