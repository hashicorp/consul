@setupApplicationTest
Feature: dc / list-blocking
  In order to see updates without refreshing the page
  As a user
  I want to see changes if I change consul externally
  Background:
    Given 1 datacenter model with the value "dc-1"
    And settings from yaml
    ---
    consul:client:
      blocking: 1
      throttle: 200
    ---
  Scenario:
    And 3 [Model] models
    And a network latency of 100
    When I visit the [Page] page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/[Url]
    And pause until I see 3 [Model] models
    And an external edit results in 5 [Model] models
    And pause until I see 5 [Model] models
    And an external edit results in 1 [Model] model
    And pause until I see 1 [Model] model
    And an external edit results in 0 [Model] models
    And pause until I see 0 [Model] models
  Where:
    --------------------------------------------
    | Page       | Model       | Url           |
    | services   | service     | services      |
    | nodes      | node        | nodes         |
    --------------------------------------------
