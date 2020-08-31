@setupApplicationTest
Feature: dc / intentions / deleting: Deleting items with confirmations, success and error notifications
  Background:
    Given 1 datacenter model with the value "datacenter"
  Scenario: Deleting a intention model from the intention listing page
    Given 1 intention model from yaml
    ---
    SourceName: name
    ID: ee52203d-989f-4f7a-ab5a-2bef004164ca
    ---
    When I visit the intentions page for yaml
    ---
      dc: datacenter
    ---
    And I click actions on the intentions
    And I click delete on the intentions
    And I click confirmDelete on the intentions
    Then a DELETE request was made to "/v1/connect/intentions/ee52203d-989f-4f7a-ab5a-2bef004164ca?dc=datacenter"
    And "[data-notification]" has the "notification-delete" class
    And "[data-notification]" has the "success" class
  Scenario: Deleting an intention from the intention detail page
    When I visit the intention page for yaml
    ---
      dc: datacenter
      intention: ee52203d-989f-4f7a-ab5a-2bef004164ca
    ---
    And I click delete
    And I click confirmDelete
    Then a DELETE request was made to "/v1/connect/intentions/ee52203d-989f-4f7a-ab5a-2bef004164ca?dc=datacenter"
    And "[data-notification]" has the "notification-delete" class
    And "[data-notification]" has the "success" class
  Scenario: Deleting an intention from the intention detail page and getting an error
    When I visit the intention page for yaml
    ---
      dc: datacenter
      intention: ee52203d-989f-4f7a-ab5a-2bef004164ca
    ---
    Given the url "/v1/connect/intentions/ee52203d-989f-4f7a-ab5a-2bef004164ca?dc=datacenter" responds with a 500 status
    And I click delete
    And I click confirmDelete
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "error" class
  Scenario: Deleting an intention from the intention detail page and getting an error due to a duplicate intention
    When I visit the intention page for yaml
    ---
      dc: datacenter
      intention: ee52203d-989f-4f7a-ab5a-2bef004164ca
    ---
    Given the url "/v1/connect/intentions/ee52203d-989f-4f7a-ab5a-2bef004164ca?dc=datacenter" responds with from yaml
    ---
      status: 500
      body: "duplicate intention found:"
    ---
    And I click delete
    And I click confirmDelete
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "error" class
    And I see the text "Intention exists" in "[data-notification] strong"
