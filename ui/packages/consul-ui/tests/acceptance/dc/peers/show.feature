@setupApplcationTest
Feature: dc / peers / show: Peers show
  Scenario: Dialer side tabs
    And 1 datacenter model with the value "dc-1"
    And 1 peer models from yaml
    ---
    Name: a-peer
    State: ACTIVE
    # dialer side
    PeerID: null
    ---
    When I visit the peers page for yaml
    ---
      dc: dc-1
    ---
    And I click actions on the peers
    And I click view on the peers
    Then the url should be /dc-1/peers/a-peer/imported-services
    Then I see importedServicesIsVisible on the tabs
    And I see exportedServicesIsVisible on the tabs
    And I don't see serverAddressesIsVisible on the tabs
  
  Scenario: Receiver side tabs
    And 1 datacenter model with the value "dc-1"
    And 1 peer models from yaml
    ---
    Name: a-peer
    State: ACTIVE
    # receiver side
    PeerID: 'some-peer'
    ---
    When I visit the peers page for yaml
    ---
      dc: dc-1
    ---
    And I click actions on the peers
    And I click view on the peers
    Then the url should be /dc-1/peers/a-peer/imported-services
    Then I see importedServicesIsVisible on the tabs
    And I see exportedServicesIsVisible on the tabs
    And I see serverAddressesIsVisible on the tabs

  Scenario: Imported Services Empty
    And 1 datacenter model with the value "dc-1"
    And 1 peer models from yaml
    ---
    Name: a-peer
    State: ACTIVE
    ---
    And 0 service models
    When I visit the peer page for yaml
    ---
      dc: dc-1
      peer: a-peer
    ---
    Then I see the "[data-test-imported-services-empty]" element
  
  Scenario: Imported Services not empty
    And 1 datacenter model with the value "dc-1"
    And 1 peer models from yaml
    ---
    Name: a-peer
    State: ACTIVE
    ---
    And 1 service models from yaml
    ---
    Name: 'service-for-peer-a'
    ---
    When I visit the peer page for yaml
    ---
      dc: dc-1
      peer: a-peer
    ---
    Then I don't see the "[data-test-imported-services-empty]" element
  
  Scenario: Exported Services Empty
    And 1 datacenter model with the value "dc-1"
    And 1 peer models from yaml
    ---
    Name: a-peer
    State: ACTIVE
    ---
    And 0 service models
    When I visitExported the peer page for yaml
    ---
      dc: dc-1
      peer: a-peer
    ---
    Then I see the "[data-test-exported-services-empty]" element
  
  Scenario: Exported Services not empty
    And 1 datacenter model with the value "dc-1"
    And 1 peer models from yaml
    ---
    Name: a-peer
    State: ACTIVE
    ---
    And 1 service models from yaml
    ---
    Name: 'service-for-peer-a'
    ---
    When I visitExported the peer page for yaml
    ---
      dc: dc-1
      peer: a-peer
    ---
    Then I don't see the "[data-test-exported-services-empty]" element
  
  Scenario: Addresses Empty
    And 1 datacenter model with the value "dc-1"
    And 1 peer models from yaml
    ---
    Name: a-peer
    State: ACTIVE
    PeerServerAddresses: null
    ---
    When I visitAddresses the peer page for yaml
    ---
      dc: dc-1
      peer: a-peer
    ---
    Then I see the "[data-test-addresses-empty]" element
  
  Scenario: Addresses Not Empty
    And 1 datacenter model with the value "dc-1"
    And 1 peer models from yaml
    ---
    Name: a-peer
    State: ACTIVE
    ---
    When I visitAddresses the peer page for yaml
    ---
      dc: dc-1
      peer: a-peer
    ---
    Then I don't see the "[data-test-addresses-empty]" element
