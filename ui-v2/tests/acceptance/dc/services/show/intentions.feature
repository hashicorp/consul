@setupApplicationTest
Feature: dc / services / intentions: Intentions per service
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
    And 3 intention models
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service-0
    ---
    And the title should be "service-0 - Consul"
    And I see intentions on the tabs
    When I click intentions on the tabs
    And I see intentionsIsSelected on the tabs
  Scenario: I can see intentions
    And I see 3 intention models
  Scenario: I can delete intentions
    And I click actions on the intentions
    And I click delete on the intentions
    And I click confirmDelete on the intentions
    Then a DELETE request was made to "/v1/connect/intentions/ee52203d-989f-4f7a-ab5a-2bef004164ca?dc=dc1"
    And "[data-notification]" has the "notification-delete" class
    And "[data-notification]" has the "success" class
