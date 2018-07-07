@setupApplicationTest
Feature: dc / intentions / form select: Intention Service Select Dropdowns
  In order to set future Consul services as intention sources and destinations
  As a user
  I want to type into the autocomplete and select what I've typed to use it as the future service
  Scenario: Selecting a future Consul Service in to [Name]
    Given 1 datacenter model with the value "datacenter"
    When I visit the intention page for yaml
    ---
      dc: datacenter
      intention: intention
    ---
    Then the url should be /datacenter/intentions/intention
    And I click "[data-test-[Name]-element] .ember-power-select-trigger"
    And I type "something" into ".ember-power-select-search-input"
    And I click ".ember-power-select-option:first-child"
    Then I see the text "something" in "[data-test-[Name]-element] .ember-power-select-selected-item"
    Where:
      ---------------
      | Name        |
      | source      |
      | destination |
      ---------------
