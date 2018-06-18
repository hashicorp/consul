@setupApplicationTest
Feature: Services
  Scenario:
    Given 1 datacenter model with the value "dc-1"
    And 3 service models
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services
    Then I see 3 service models
