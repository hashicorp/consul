@setupApplicationTest
Feature: dc / kvs / deleting: Deleting items with confirmations, success and error notifications
  Background:
    Given 1 datacenter model with the value "datacenter"
  Scenario: Deleting a kv model from the kv listing page
    Given 1 kv model from yaml
    ---
      ["key-name"]
    ---
    When I visit the kvs page for yaml
    ---
      dc: datacenter
    ---
    And I click actions on the kvs
    And I click delete on the kvs
    And I click confirmDelete on the kvs
    Then a DELETE request was made to "/v1/kv/key-name?dc=datacenter&ns=@!namespace"
    And "[data-notification]" has the "notification-delete" class
    And "[data-notification]" has the "success" class
  Scenario: Deleting an kv from the kv detail page
    When I visit the kv page for yaml
    ---
      dc: datacenter
      kv: key-name
    ---
    And I click delete
    And I click confirmDelete
    Then a DELETE request was made to "/v1/kv/key-name?dc=datacenter&ns=@!namespace"
    And "[data-notification]" has the "notification-delete" class
    And "[data-notification]" has the "success" class
  Scenario: Deleting an kv from the kv detail page and getting an error
    When I visit the kv page for yaml
    ---
      dc: datacenter
      kv: key-name
    ---
    Given the url "/v1/kv/key-name?dc=datacenter&ns=@!namespace" responds with a 500 status
    And I click delete
    And I click confirmDelete
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "error" class

