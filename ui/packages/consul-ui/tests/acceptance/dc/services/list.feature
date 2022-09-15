@setupApplicationTest
Feature: dc / services / list
  Scenario: Listing service
    Given 1 datacenter model with the value "dc-1"
    And 3 service models from yaml
    ---
      - Name: Service-0
        Kind: ~
      - Name: Service-1
        Kind: ~
      - Name: Service-2
        Kind: ~
    ---
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services
    And a GET request was made to "/v1/internal/ui/services?dc=dc-1&ns=@namespace"

    Then I see 3 service models


  Scenario: Listing peered service
    Given 1 datacenter model with the value "dc-1"
    And 1 service models from yaml
      ---
        - Name: Service-0
          Kind: ~
          PeerName: billing-app
      ---
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services

    Then I see 1 service model with the peer "billing-app"
