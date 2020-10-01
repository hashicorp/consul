@setupApplicationTest
Feature: dc / services / dc-switch : Switching Datacenters
  Scenario: Seeing all services when switching datacenters
    Given 2 datacenter models from yaml
    ---
      - dc-1
      - dc-2
    ---
    And 6 service models from yaml
    ---
      - Name: Service-0
        Kind: ~
      - Name: Service-1
        Kind: ~
      - Name: Service-2
        Kind: ~
      - Name: Service-3
        Kind: ~
      - Name: Service-4
        Kind: ~
      - Name: Service-5
        Kind: ~
    ---

    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services
    Then I see 6 service models
    When I click dc on the navigation
    And I click dcs.1.name on the navigation
    Then the url should be /dc-2/services
    Then I see 6 service models
    When I click dc on the navigation
    And I click dcs.0.name on the navigation
    Then the url should be /dc-1/services
    Then I see 6 service models
    When I click dc on the navigation
    And I click dcs.1.name on the navigation
    Then the url should be /dc-2/services
    Then I see 6 service models
