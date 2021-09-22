@setupApplicationTest
Feature: dc / intentions / filtered-select: Intention Service Select Dropdowns
  In order to use services as intention sources and destinations
  As a user
  I want to be able to choose see existing services in the dropdown, but not existing proxy services
  Scenario: Opening the [Name] dropdown with 2 services and 2 proxy services
    Given 1 datacenter model with the value "datacenter"
    And 4 service models from yaml
    ---
    - Name: service-0
      Kind: ~
    - Name: service-1
      Kind: ~
    - Name: service-2
      Kind: connect-proxy
    - Name: service-3
      Kind: connect-proxy
    ---
    When I visit the intention page for yaml
    ---
      dc: datacenter
    ---
    Then the url should be /datacenter/intentions/create
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
  @onlyNamespaceable
  Scenario: Opening and closing the nspace [Name] dropdown doesn't double up items
    Given 1 datacenter model with the value "datacenter"
    And 2 nspace models from yaml
    ---
    - Name: nspace-0
    - Name: nspace-1
    ---
    When I visit the intention page for yaml
    ---
      dc: datacenter
    ---
    Then the url should be /datacenter/intentions/create
    Given a network latency of 60000
    And I click "[data-test-[Name]-nspace] .ember-power-select-trigger"
    Then I see the text "* (All Namespaces)" in ".ember-power-select-option:nth-last-child(3)"
    Then I see the text "nspace-0" in ".ember-power-select-option:nth-last-child(2)"
    Then I see the text "nspace-1" in ".ember-power-select-option:last-child"
    And I click ".ember-power-select-option:last-child"
    And I click "[data-test-[Name]-nspace] .ember-power-select-trigger"
    And I click ".ember-power-select-option:last-child"
    And I click "[data-test-[Name]-nspace] .ember-power-select-trigger"
    And I click ".ember-power-select-option:last-child"
    And I click "[data-test-[Name]-nspace] .ember-power-select-trigger"
    Then I don't see the text "nspace-1" in ".ember-power-select-option:nth-last-child(6)"
    Then I don't see the text "nspace-1" in ".ember-power-select-option:nth-last-child(5)"
    Then I don't see the text "nspace-1" in ".ember-power-select-option:nth-last-child(4)"
    Then I see the text "* (All Namespaces)" in ".ember-power-select-option:nth-last-child(3)"
    Then I see the text "nspace-0" in ".ember-power-select-option:nth-last-child(2)"
    Then I see the text "nspace-1" in ".ember-power-select-option:last-child"
    Where:
      ---------------
      | Name        |
      | source      |
      | destination |
      ---------------
  Scenario: Opening the [Name] dropdown with 2 services with the same name from different nspaces
    Given 1 datacenter model with the value "datacenter"
    And 2 service models from yaml
    ---
    - Name: service-0
      Kind: ~
    - Name: service-0
      Namespace: nspace
      Kind: ~
    ---
    When I visit the intention page for yaml
    ---
      dc: datacenter
    ---
    Then the url should be /datacenter/intentions/create
    And I click "[data-test-[Name]-element] .ember-power-select-trigger"
    Then I see the text "* (All Services)" in ".ember-power-select-option:nth-last-child(2)"
    Then I see the text "service-0" in ".ember-power-select-option:last-child"
    Where:
      ---------------
      | Name        |
      | source      |
      | destination |
      ---------------
