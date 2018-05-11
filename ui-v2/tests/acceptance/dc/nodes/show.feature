@setupApplicationTest
Feature: Show node
  Scenario: Given 2 nodes all the tabs are visible and clickable
    Given 1 datacenter model with the value "dc1"
    And 2 node models from yaml
    ---
    - ID: node-0
    - ID: node-1
    ---
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
@ignore
  Scenario: Given 1 node all the tabs are visible and clickable and the RTT one isn't there
    Given 1 datacenter model with the value "dc1"
    And 1 node models from yaml
    ---
    - ID: node-0
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

