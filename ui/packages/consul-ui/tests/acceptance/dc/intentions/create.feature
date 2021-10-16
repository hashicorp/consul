@setupApplicationTest
Feature: dc / intentions / create: Intention Create
  In order to define intentions
  As a user
  I want to visit the intention create page, fill in the form and hit the create button and see a success notification
  @onlyNamespaceable
  Scenario: with namespaces enabled
    Given 1 datacenter model with the value "datacenter"
    And 3 service models from yaml
    ---
    - Name: web
      Kind: ~
    - Name: db
      Kind: ~
    - Name: cache
      Kind: ~
    ---
    And 1 nspace model from yaml
    ---
    - Name: nspace-0
    ---
    When I visit the intention page for yaml
    ---
      dc: datacenter
    ---
    Then the url should be /datacenter/intentions/create
    And the title should be "New Intention - Consul"
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
    # Set source nspace
    And I click "[data-test-source-nspace] .ember-power-select-trigger"
    And I click ".ember-power-select-option:last-child"
    Then I see the text "nspace-0" in "[data-test-source-nspace] .ember-power-select-selected-item"
    # Set destination nspace
    And I click "[data-test-destination-nspace] .ember-power-select-trigger"
    And I click ".ember-power-select-option:last-child"
    Then I see the text "nspace-0" in "[data-test-destination-nspace] .ember-power-select-selected-item"
    # Specifically set deny
    And I click ".value-deny"
    And I submit
    Then a PUT request was made to "/v1/connect/intentions/exact?source=default%2Fnspace-0%2Fweb&destination=default%2Fnspace-0%2Fdb&dc=datacenter" from yaml
    ---
      body:
        SourceName: web
        DestinationName: db
        SourceNS: nspace-0
        DestinationNS: nspace-0
        SourcePartition: default
        DestinationPartition: default
        Action: deny
    ---
    Then the url should be /datacenter/intentions
    And the title should be "Intentions - Consul"
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
  @notNamespaceable
  Scenario: with namespaces disabled
    Given 1 datacenter model with the value "datacenter"
    And 3 service models from yaml
    ---
    - Name: web
      Kind: ~
    - Name: db
      Kind: ~
    - Name: cache
      Kind: ~
    ---
    When I visit the intention page for yaml
    ---
      dc: datacenter
    ---
    Then the url should be /datacenter/intentions/create
    And the title should be "New Intention - Consul"
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
    And I click ".value-deny"
    And I submit
    Then a PUT request was made to "/v1/connect/intentions/exact?source=default%2Fdefault%2Fweb&destination=default%2Fdefault%2Fdb&dc=datacenter" from yaml
    ---
      body:
        SourceName: web
        DestinationName: db
        Action: deny
    ---
    Then the url should be /datacenter/intentions
    And the title should be "Intentions - Consul"
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
