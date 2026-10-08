@setupApplicationTest
Feature: dc / peers / create: Peer Create Token
  Background:
    Given 1 datacenter model with the value "dc-1"

  Scenario: Visiting the Add peer connection page directly
    When I visit the peers page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/peers
    And I click create
    Then the url should be /dc-1/peers/create
    And the title should be "Add peer connection - Consul"

  Scenario: Generating a token from the Generate token tab
    Given the url "/v1/peering/token" responds with from yaml
    ---
    body:
      PeeringToken: an-encoded-token
    ---
    When I visit the peerCreate page for yaml
    ---
      dc: dc-1
    ---
    Then I fill in with yaml
    ---
      Name: new-peer
    ---
    And I submit
    Then a POST request was made to "/v1/peering/token" from yaml
    ---
      body:
        PeerName: new-peer
        ServerExternalAddresses: []
    ---
    Then I see the text "an-encoded-token" in "#copy-token-modal [data-test-peering-token]"

  Scenario: Establishing a peering from the Establish peering tab
    Given the url "/v1/peering/establish" responds with from yaml
    ---
    body:
      Status: 200
    ---
    When I visit the peerCreate page for yaml
    ---
      dc: dc-1
    ---
    And I click tabs.initiate
    Then I fill in with yaml
    ---
      Name: new-peer
      PeeringToken: an-encoded-token
    ---
    And I submit
    Then a POST request was made to "/v1/peering/establish" from yaml
    ---
      body:
        PeerName: new-peer
        PeeringToken: an-encoded-token
    ---
    Then the url should be /dc-1/peers/new-peer/imported-services

  Scenario: Cancelling with unsaved input asks for confirmation
    When I visit the peerCreate page for yaml
    ---
      dc: dc-1
    ---
    Then I fill in with yaml
    ---
      Name: new-peer
    ---
    And I click cancel
    Then the url should be /dc-1/peers/create
    And I click keepEditing
    Then the url should be /dc-1/peers/create
    When I click cancel
    And I click confirmDiscard
    Then the url should be /dc-1/peers

  Scenario: Cancelling an empty form does not ask for confirmation
    When I visit the peerCreate page for yaml
    ---
      dc: dc-1
    ---
    And I click cancel
    Then the url should be /dc-1/peers
