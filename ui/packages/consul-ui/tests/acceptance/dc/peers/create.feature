@setupApplicationTest
Feature: dc / peers / create: Peer Create Token
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
    Then I fill in with yaml
    ---
      Name: new-peer
    ---
    When I click ".peer-create-modal .modal-dialog-footer button"
    Then a POST request was made to "/v1/peering/token" from yaml
    ---
      body:
        PeerName: new-peer
        ServerExternalAddresses: []
    ---
    Then I see the text "an-encoded-token" in ".consul-peer-form-generate code"
    When I click ".consul-peer-form-generate button[type=reset]"
    And the url "/v1/peering/token" responds with from yaml
    ---
    body:
      PeeringToken: another-encoded-token
    ---
    Then I fill in with yaml
    ---
      Name: another-new-peer
      ServerExternalAddresses: "1.1.1.1:123,1.2.3.4:3202"
    ---
    When I click ".peer-create-modal .modal-dialog-footer button"
    Then a POST request was made to "/v1/peering/token" from yaml
    ---
      body:
        PeerName: another-new-peer
        ServerExternalAddresses: ["1.1.1.1:123","1.2.3.4:3202"]
    ---
    Then I see the text "another-encoded-token" in ".consul-peer-form-generate code"
