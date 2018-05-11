@setupApplicationTest
Feature: index forwarding
  Scenario: Arriving at the index page when there is only one datacenter
    Given 1 datacenter model with the value "datacenter"
    When I visit the index page
    Then the url should be /datacenter/services
