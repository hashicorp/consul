@setupApplicationTest
Feature: dc / services / list
  Scenario: Listing service
    Given 1 datacenter model with the value "dc-1"
    And 3 service models from yaml
    ---
      - Name: Service-0
        Kind: ~
      - Name: Service-1
        Kind: ~
      - Name: Service-2
        Kind: ~
    ---
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services

    Then I see 3 service models