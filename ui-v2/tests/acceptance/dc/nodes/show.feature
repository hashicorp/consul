@setupApplicationTest
Feature: dc / nodes / show: Show node
  Background:
    Given 1 datacenter model with the value "dc1"
  Scenario: Given 2 nodes all the tabs are visible and clickable
    Given 2 node models from yaml
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

    When I click metaData on the tabs
    And I see metaDataIsSelected on the tabs
  Scenario: Given 1 node all the tabs are visible and clickable and the RTT one isn't there
    Given 1 node models from yaml
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
    Given 1 node models from yaml
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
  Scenario: A node warns when deregistered whilst blocking
    Given 1 node model from yaml
    ---
    ID: node-0
    ---
    And settings from yaml
    ---
    consul:client:
      blocking: 1
      throttle: 200
    ---
    And a network latency of 100
    When I visit the node page for yaml
    ---
      dc: dc1
      node: node-0
    ---
    Then the url should be /dc1/nodes/node-0
    And the url "/v1/internal/ui/node/node-0" responds with a 404 status
    And pause until I see the text "no longer exists" in "[data-notification]"
  @ignore
    Scenario: The RTT for the node is displayed properly
    Then ok
  @ignore
    Scenario: The RTT for the node displays properly whilst blocking
    Then ok

