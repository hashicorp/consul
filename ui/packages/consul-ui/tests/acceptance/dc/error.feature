@setupApplicationTest
Feature: dc / error
  Scenario: Recovering from a dc 500 error
    Given 2 datacenter models from yaml
    ---
    - dc-1
    - dc-500
    ---
    And 3 service models from yaml
    ---
      - Name: Service-0
        Kind: ~
      - Name: Service-1
        Kind: ~
      - Name: Service-2
        Kind: ~
    ---
    And the url "/v1/internal/ui/services" responds with a 500 status
    When I visit the services page for yaml
    ---
      dc: dc-500
    ---
    Then the url should be /dc-500/services
    And the title should be "Consul"
    Then I see status on the error like "500"
    And the url "/v1/internal/ui/services" responds with a 200 status
    When I click dc on the navigation
    And I click dcs.0.name on the navigation
    Then I see 3 service models
