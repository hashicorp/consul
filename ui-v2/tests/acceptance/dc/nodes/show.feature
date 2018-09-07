@setupApplicationTest
Feature: dc / nodes / show: Show node
  Scenario: Given 2 nodes all the tabs are visible and clickable
    Given 1 datacenter model with the value "dc1"
    And 2 node models from yaml
    When I visit the node page for yaml
    ---
      dc: dc1
      node: node-0
    ---
    And I see healthChecksIsSelected on the tabs

    When I click services on the tabs
    And I see servicesIsSelected on the tabs

    When I click roundTripTime on the tabs
    And I see roundTripTimeIsSelected on the tabs

    When I click lockSessions on the tabs
    And I see lockSessionsIsSelected on the tabs
  Scenario: Given 1 node all the tabs are visible and clickable and the RTT one isn't there
    Given 1 datacenter model with the value "dc1"
    And 1 node models from yaml
    ---
    ID: node-0
    ---
    When I visit the node page for yaml
    ---
      dc: dc1
      node: node-0
    ---
    And I see healthChecksIsSelected on the tabs

    When I click services on the tabs
    And I see servicesIsSelected on the tabs

    And I don't see roundTripTime on the tabs

    When I click lockSessions on the tabs
    And I see lockSessionsIsSelected on the tabs
  Scenario: Given 1 node with no checks all the tabs are visible but the Services tab is selected
    Given 1 datacenter model with the value "dc1"
    And 1 node models from yaml
    ---
    ID: node-0
    Checks: []
    ---
    When I visit the node page for yaml
    ---
      dc: dc1
      node: node-0
    ---
    And I see healthChecks on the tabs
    And I see services on the tabs
    And I see roundTripTime on the tabs
    And I see lockSessions on the tabs
    And I see servicesIsSelected on the tabs
