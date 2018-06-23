@setupApplicationTest
Feature: Datacenters
@ignore
  Scenario: Arriving at the service page
    Given 10 datacenter models
    When I visit the index page
    And I click "[data-test-datacenter-selected]"
    Then I see 10 datacenter models
