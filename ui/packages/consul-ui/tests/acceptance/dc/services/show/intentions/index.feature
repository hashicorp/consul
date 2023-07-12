@setupApplicationTest
Feature: dc / services / show / intentions / index: Intentions per service
  Background:
    Given 1 datacenter model with the value "dc1"
    And 1 node models
    And 1 service model from yaml
    ---
    - Service:
        Kind: ~
        Name: service-0
        ID: service-0-with-id
    ---
    And 3 intention models from yaml
    ---
    - ID: 755b72bd-f5ab-4c92-90cc-bed0e7d8e9f0
      Action: allow
      Meta: ~
      SourceName: name
      DestinationName: destination
      SourceNS: default
      DestinationNS: default
      SourcePartition: default
      DestinationPartition: default
      SourcePeer: ""

    - ID: 755b72bd-f5ab-4c92-90cc-bed0e7d8e9f1
      Action: deny
      Meta: ~
      SourcePeer: ""
    - ID: 0755b72bd-f5ab-4c92-90cc-bed0e7d8e9f2
      Action: deny
      Meta: ~
      SourcePeer: ""
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service-0
    ---
    And the title should be "service-0 - Consul"
    And I see intentionsIsVisible on the tabs
    When I click intentions on the tabs
    And I see intentionsIsSelected on the tabs
  Scenario: I can see intentions
    And I see 3 intention models on the intentionList component
    And I click intention on the intentionList.intentions component
    Then the url should be /dc1/services/service-0/intentions/default:default:name:default:default:destination
  Scenario: I can delete intentions
    And I click actions on the intentionList.intentions component
    And I click delete on the intentionList.intentions component
    And I click confirmDelete on the intentionList.intentions
    Then a DELETE request was made to "/v1/connect/intentions/exact?source=default%2Fdefault%2Fname&destination=default%2Fdefault%2Fdestination&dc=dc1"
    And "[data-notification]" has the "notification-delete" class
    And "[data-notification]" has the "success" class
