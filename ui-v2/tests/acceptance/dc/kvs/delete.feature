@setupApplicationTest
Feature: dc / kvs / delete: KV Delete
  Scenario: Delete ACL
    Given 1 datacenter model with the value "datacenter"
    And 1 kv model from yaml
    ---
      - key-name
    ---
    When I visit the kvs page for yaml
    ---
      dc: datacenter
    ---
    And I click actions on the kvs
    And I click delete on the kvs
    And I click confirmDelete on the kvs
    Then a DELETE request is made to "/v1/kv/key-name?dc=datacenter"
