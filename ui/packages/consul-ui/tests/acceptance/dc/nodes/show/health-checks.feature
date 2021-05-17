@setupApplicationTest
Feature: dc / nodes / show / health-checks
  Background:
    Given 1 datacenter model with the value "dc1"
  Scenario: A failing serf check
    Given 1 node model from yaml
    ---
    ID: node-0
    Checks:
      - Type: ''
        Name: Serf Health Status
        CheckID: serfHealth
        Status: critical
        Output: ouch
    ---
    When I visit the node page for yaml
    ---
      dc: dc1
      node: node-0
    ---
    And I see healthChecksIsSelected on the tabs
    And I see criticalSerfNotice on the tabs.healthChecksTab
  Scenario: A passing serf check
    Given 1 node model from yaml
    ---
    ID: node-0
    Checks:
      - Type: ''
        Name: Serf Health Status
        CheckID: serfHealth
        Status: passing
        Output: Agent alive and reachable
    ---
    When I visit the node page for yaml
    ---
      dc: dc1
      node: node-0
    ---
    And I see healthChecksIsSelected on the tabs
    And I don't see criticalSerfNotice on the tabs.healthChecksTab
