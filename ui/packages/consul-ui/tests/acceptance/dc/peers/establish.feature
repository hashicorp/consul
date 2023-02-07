@setupApplicationTest
Feature: dc / peers / establish: Peer Establish Peering
  Scenario:
    Given 1 datacenter model with the value "dc-1"
    And the url "/v1/peering/token" responds with from yaml
    ---
    body:
      PeeringToken: an-encoded-token
    ---
    When I visit the peers page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/peers
    And I click create
    When I click "[data-test-tab=tab_establish-peering] button"
    Then I fill in with yaml
    ---
      Name: new-peer
      Token: an-encoded-token
    ---
    When I click ".peer-create-modal .modal-dialog-footer button"
    Then a POST request was made to "/v1/peering/establish" from yaml
    ---
      body:
        PeerName: new-peer
        PeeringToken: an-encoded-token
    ---
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
    And the url should be /dc-1/peers/new-peer/imported-services
