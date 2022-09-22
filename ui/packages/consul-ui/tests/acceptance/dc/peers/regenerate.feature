@setupApplicationTest
Feature: dc / peers / regenerate: Regenerate Peer Token
  Scenario:
    Given 1 datacenter model with the value "datacenter"
    And 1 peer model from yaml
    ---
    Name: peer-name
    State: ACTIVE
    ---
    And the url "/v1/peering/token" responds with from yaml
    ---
    body:
      PeeringToken: an-encoded-token
    ---
    When I visit the peers page for yaml
    ---
      dc: datacenter
    ---
    And I click actions on the peers
    And I click regenerate on the peers
    Then I see the text "an-encoded-token" in ".consul-peer-form-generate code"
