@setupApplicationTest
Feature: dc / services / show / navigation
  Scenario: Accessing peered service directly
    Given 1 datacenter model with the value "dc-1"
    And 1 service models
    When I visit the service page with the url /dc-1/services/peer:billing:service-0
    Then I see peer like "billing"
