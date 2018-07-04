@setupApplicationTest
Feature: dc / nodes / sessions / invalidate: Invalidate Lock Sessions
  In order to invalidate a lock session
  As a user
  I should be able to invalidate a lock session by clicking a button and confirming
  Scenario: Given 2 lock sessions
    Given 1 datacenter model with the value "dc1"
    And 1 node model from yaml
    ---
    - ID: node-0
    ---
    And 2 session models from yaml
    ---
    - ID: 7bbbd8bb-fff3-4292-b6e3-cfedd788546a
    - ID: 7ccd0bd7-a5e0-41ae-a33e-ed3793d803b2
    ---
    When I visit the node page for yaml
    ---
      dc: dc1
      node: node-0
    ---
    And I click lockSessions on the tabs
    Then I see lockSessionsIsSelected on the tabs
    And I click delete on the sessions
    And I click confirmDelete on the sessions
    Then a PUT request is made to "/v1/session/destroy/7bbbd8bb-fff3-4292-b6e3-cfedd788546a?dc=dc1"
