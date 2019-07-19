@setupApplicationTest
Feature: dc / services / instances / error: Visit Service Instance what doesn't exist
  Scenario: No instance can be found in the API response
    Given 1 datacenter model with the value "dc1"
    And 1 instance model
    When I visit the instance page for yaml
    ---
      dc: dc1
      service: service-0
      node: node-0
      id: id-that-doesnt-exist
    ---
    Then the url should be /dc1/services/service-0/node-0/id-that-doesnt-exist
    And I see the text "404 (Unable to find instance)" in "[data-test-error]"


