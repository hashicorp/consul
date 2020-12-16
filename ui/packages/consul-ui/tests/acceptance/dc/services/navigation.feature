@setupApplicationTest
Feature: dc / services / navigation
  Scenario: Clicking a service in the listing and back again
    Given 1 datacenter model with the value "dc-1"
    And 1 service model
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services
    And the title should be "Services - Consul"
    Then I see 1 service models
    When I click service on the services
    And I click "[data-test-back]"
    Then the url should be /dc-1/services

