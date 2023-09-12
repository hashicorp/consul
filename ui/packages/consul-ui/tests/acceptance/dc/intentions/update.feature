@setupApplicationTest
Feature: dc / intentions / update: Intention Update
  Background:
    Given 1 datacenter model with the value "datacenter"
    And 1 intention model from yaml
    ---
      SourceName: web
      DestinationName: db
      SourceNS: default
      DestinationNS: default
      SourcePartition: default
      DestinationPartition: default
      ID: intention-id
    ---
    When I visit the intention page for yaml
    ---
      dc: datacenter
      intention: intention-id
    ---
    Then the url should be /datacenter/intentions/intention-id
    And the title should be "Edit Intention - Consul"
  Scenario: Update to [Description], [Action]
    Then I fill in with yaml
    ---
      Description: [Description]
    ---
    And I click "[value=[Action]]"
    And I submit
    Then a PUT request was made to "/v1/connect/intentions/exact?source=default%2Fdefault%2Fweb&destination=default%2Fdefault%2Fdb&dc=datacenter" from yaml
    ---
      Description: [Description]
      Action: [Action]
    ---
    Then the url should be /datacenter/intentions
    And the title should be "Intentions - Consul"
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
    Where:
      ------------------------------
      | Description       | Action |
      | Desc              | allow  |
      ------------------------------
  Scenario: There was an error saving the intention
    Given the url "/v1/connect/intentions/exact?source=default%2Fdefault%2Fweb&destination=default%2Fdefault%2Fdb&dc=datacenter" responds with a 500 status
    And I submit
    Then the url should be /datacenter/intentions/intention-id
    Then "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "error" class

