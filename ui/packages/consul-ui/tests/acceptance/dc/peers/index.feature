@setupApplicationTest
Feature: dc / peers / index: Peers List
  Background:
    And 1 datacenter model with the value "dc-1"
    And 3 peer models from yaml
    ---
    - Name:  a-peer
    - Name:  b-peer
    - Name:  z-peer
    ---
    When I visit the peers page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/peers
    And the title should be "Peers - Consul"
  Scenario:
    Then I see 3 peer models

