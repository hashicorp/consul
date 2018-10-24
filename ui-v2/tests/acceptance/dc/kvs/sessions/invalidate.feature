@setupApplicationTest
Feature: dc / kvs / sessions / invalidate: Invalidate Lock Sessions
  In order to invalidate a lock session
  As a user
  I should be able to invalidate a lock session by clicking a button and confirming
  Background:
    Given 1 datacenter model with the value "datacenter"
    And 1 kv model from yaml
    ---
      Key: key
    ---
    When I visit the kv page for yaml
    ---
      dc: datacenter
      kv: key
    ---
    Then the url should be /datacenter/kv/key/edit

  Scenario: Invalidating the lock session
    And I click delete on the session
    And I click confirmDelete on the session
    Then the last PUT request was made to "/v1/session/destroy/ee52203d-989f-4f7a-ab5a-2bef004164ca?dc=datacenter"
    Then the url should be /datacenter/kv/key/edit
    And "[data-notification]" has the "notification-delete" class
    And "[data-notification]" has the "success" class
  Scenario: Invalidating a lock session and receiving an error
    Given the url "/v1/session/destroy/ee52203d-989f-4f7a-ab5a-2bef004164ca?dc=datacenter" responds with a 500 status
    And I click delete on the session
    And I click confirmDelete on the session
    Then the url should be /datacenter/kv/key/edit
    And "[data-notification]" has the "notification-delete" class
    And "[data-notification]" has the "error" class
