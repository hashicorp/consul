@setupApplicationTest
Feature: dc / peers / delete: Deleting items with confirmations, success and error notifications
  Background:
    Given 1 datacenter model with the value "datacenter"
  Scenario: Deleting a peer model from the listing page
    Given 1 peer model from yaml
    ---
    Name: peer-name
    State: ACTIVE
    ---
    When I visit the peers page for yaml
    ---
      dc: datacenter
    ---
    And I click actions on the peers
    And I click delete on the peers
    And I click confirmDelete on the peers
    Then a DELETE request was made to "/v1/peering/peer-name"
    And "[data-notification]" has the "notification-delete" class
    And "[data-notification]" has the "success" class
  Scenario: Deleting a peer from the peer listing page with error
    Given 1 peer model from yaml
    ---
    Name: peer-name
    State: ACTIVE
    ---
    When I visit the peers page for yaml
    ---
      dc: datacenter
    ---
    Given the url "/v1/peering/peer-name" responds with a 500 status
    And I click actions on the peers
    And I click delete on the peers
    And I click confirmDelete on the peers
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "error" class
  Scenario: A Peer currently deleting cannot be deleted
    Given 1 peer model from yaml
    ---
    Name: peer-name
    State: DELETING
    ---
    When I visit the peers page for yaml
    ---
      dc: datacenter
    ---
    And I don't see actions on the peers
