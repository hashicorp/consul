@setupApplicationTest
Feature: dc / error: Recovering from a dc 500 error
  Background:
    Given 2 datacenter models from yaml
    ---
    - dc-1
    - dc-500
    ---
    And 3 service models
    And the url "/v1/internal/ui/services" responds with a 500 status
    When I visit the services page for yaml
    ---
      dc: dc-500
    ---
    Then the url should be /dc-500/services
    And the title should be "Consul"
    Then I see the text "500 (The backend responded with an error)" in "[data-test-error]"
  Scenario: Clicking the back to root button
    Given the url "/v1/internal/ui/services" responds with a 200 status
    When I click home
    Then I see 3 service models
  Scenario: Choosing a different dc from the dc menu
    Given the url "/v1/internal/ui/services" responds with a 200 status
    When I click dc on the navigation
    And I click dcs.0.name
    Then I see 3 service models
