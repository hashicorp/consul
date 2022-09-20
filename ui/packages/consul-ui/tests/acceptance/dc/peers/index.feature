@setupApplicationTest
Feature: dc / peers / index: Peers List
  Background:
    And 1 datacenter model with the value "dc-1"
    And 3 peer models from yaml
    ---
    - Name:  z-peer
    - Name:  b-peer
    - Name:  a-peer
    ---
    When I visit the peers page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/peers
    And the title should be "Peers - Consul"
  Scenario: Viewing peers
    Then I see 3 peer models

  Scenario: Sorting peers
    When I click selected on the sort
    # alpha
    When I click options.2.button on the sort
    Then I see name on the peers vertically like yaml
    ---
    - a-peer
    - b-peer
    - z-peer
    ---
  Scenario: Searching peers
    Then I fill in with yaml
    ---
    s: a-peer
    ---
    And I see 1 peer model
    And I see 1 peer model with the name "a-peer"

