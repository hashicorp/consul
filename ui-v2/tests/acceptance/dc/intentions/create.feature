@setupApplicationTest
Feature: dc / intentions / update: Intention Create
  In order to define intentions
  As a user
  I want to visit the intention create page, fill in the form and hit the create button and see a success notification
  Scenario:
    Given 1 datacenter model with the value "datacenter"
    When I visit the intention page for yaml
    ---
      dc: datacenter
    ---
    Then the url should be /datacenter/intentions/create
    # Set source
    And I click "[data-test-source-element] .ember-power-select-trigger"
    And I type "web" into ".ember-power-select-search-input"
    And I click ".ember-power-select-option:first-child"
    Then I see the text "web" in "[data-test-source-element] .ember-power-select-selected-item"
    # Set destination
    And I click "[data-test-destination-element] .ember-power-select-trigger"
    And I type "db" into ".ember-power-select-search-input"
    And I click ".ember-power-select-option:first-child"
    Then I see the text "db" in "[data-test-destination-element] .ember-power-select-selected-item"
    # Specifically set deny
    And I click "[value=deny]"
    And I submit
    Then a POST request is made to "/v1/connect/intentions?dc=datacenter" with the body from yaml
    ---
      SourceName: web
      DestinationName: db
      Action: deny
    ---
    Then the url should be /datacenter/intentions
    And "[data-notification]" has the "notification-create" class
    And "[data-notification]" has the "success" class
