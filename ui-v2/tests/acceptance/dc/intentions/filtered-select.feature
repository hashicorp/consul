@setupApplicationTest
Feature: dc / intentions / filtered select: Intention Service Select Dropdowns
  In order to use services as intention sources and destinations
  As a user
  I want to be able to choose see existing services in the dropdown, but not existing proxy services
  Scenario: Opening the [Name] dropdown with 2 services and 2 proxy services
    Given 1 datacenter model with the value "datacenter"
    And 4 service models from yaml
    ---
    - Name: service-0
      Kind: consul
    - Name: service-1
      Kind: consul
    - Name: service-2
      Kind: connect-proxy
    - Name: service-3
      Kind: connect-proxy
    ---
    When I visit the intention page for yaml
    ---
      dc: datacenter
      intention: intention
    ---
    Then the url should be /datacenter/intentions/intention
    And I click "[data-test-[Name]-element] .ember-power-select-trigger"
    Then I see the text "* (All Services)" in ".ember-power-select-option:nth-last-child(3)"
    Then I see the text "service-0" in ".ember-power-select-option:nth-last-child(2)"
    Then I see the text "service-1" in ".ember-power-select-option:last-child"
    Where:
      ---------------
      | Name        |
      | source      |
      | destination |
      ---------------
