@setupApplicationTest
Feature: dc / peers / sorting
  Scenario: Sorting Peers from the toolbar dropdown
    Given 1 datacenter model with the value "dc-1"
    And 3 peer models from yaml
    ---
    - Name: z-peer
    - Name: b-peer
    - Name: a-peer
    ---
    When I visit the peers page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/peers
    Then I see 3 peer models
    When I click selected on the sort
    When I click options.1.button on the sort
    Then I see name on the peers vertically like yaml
    ---
    - z-peer
    - b-peer
    - a-peer
    ---
    When I click selected on the sort
    When I click options.0.button on the sort
    Then I see name on the peers vertically like yaml
    ---
    - a-peer
    - b-peer
    - z-peer
    ---
