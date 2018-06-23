@setupApplicationTest
Feature: dc / services / error
  Scenario: Arriving at the service page that doesn't exist
    Given 2 datacenter models from yaml
    ---
    - dc-1
    - dc-2
    ---
    When I visit the services page for yaml
    ---
      dc: 404-datacenter
    ---
    Then I see the text "404 (Page not found)" in "[data-test-error]"
  Scenario: Arriving at the service page
    Given 2 datacenter models from yaml
    ---
    - dc-1
    - dc-2
    ---
    Given the url "/v1/internal/ui/services" responds with a 500 status
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then I see the text "500 (The backend responded with an error)" in "[data-test-error]"
    And I click "[data-test-datacenter-selected]"
    And I see 2 datacenter models
