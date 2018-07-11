@setupApplicationTest
Feature: dc / kvs / edit create keynames
  In order to navigate to keynames/paths ending in edit or create
  As a user
  I should be able to visit kvs with paths ending in edit/ or create/ and see the folder listing
  Background:
    Given 1 datacenter model with the value "datacenter"
  Scenario: I have 1 key in the foo/bar/edit/ folder
    And 1 kv model from yaml
    ---
      - edit
    ---
    When I visit the kvs page for yaml
    ---
      dc: datacenter
      kv: foo/bar/edit/
    ---
    Then the url should be /datacenter/kv/foo/bar/edit/
    And I see 1 kv model
    # And the last GET request was made to "/v1/kv/foo/bar/edit/?keys&dc=datacenter&separator=%2F"
  Scenario: I have 1 key in the foo/bar/create/ folder
    And 1 kv model from yaml
    ---
      - create
    ---
    When I visit the kvs page for yaml
    ---
      dc: datacenter
      kv: foo/bar/create/
    ---
    Then the url should be /datacenter/kv/foo/bar/create/
    And I see 1 kv model
    # And the last GET request was made to "/v1/kv/foo/bar/create/?keys&dc=datacenter&separator=%2F"
