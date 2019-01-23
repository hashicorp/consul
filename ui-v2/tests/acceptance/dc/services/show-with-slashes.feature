@setupApplicationTest
Feature: dc / services / show-with-slashes: Show Service that has slashes in its name
  In order to view services that have slashes in their name
  As a user
  I want to view the service in the service listing and click on it to see the service detail
  Scenario: Given a service with slashes in its name
    Given 1 datacenter model with the value "dc1"
    And 1 node model
    And 1 service model from yaml
    ---
    - Name: hashicorp/service/service-0
    ---
    When I visit the services page for yaml
    ---
      dc: dc1
    ---
    Then the url should be /dc1/services
    Then I see 1 service model
    And I click service on the services
    Then the url should be /dc1/services/hashicorp%2Fservice%2Fservice-0

