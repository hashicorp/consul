@setupApplicationTest
@notNamespaceable
Feature: index-forwarding
  Scenario: Arriving at the index page when there is only one datacenter
    Given 1 datacenter model with the value "dc1"
    When I visit the index page
    Then the url should be /dc1/services
