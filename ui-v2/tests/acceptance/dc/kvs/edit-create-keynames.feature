@setupApplicationTest
Feature: dc / kvs / edit create keynames
  In order to navigate to keynames/paths ending in edit or create
  As a user
  I should be able to visit kvs with paths ending in edit/ or create/ and see the folder listing
  Background:
    Given 1 datacenter model with the value "datacenter"
  Scenario: I have 1 key in the foo/bar/[Action]/ folder and I visit with a trailing slash
    Given 1 kv model from yaml
    ---
      - [Action]
    ---
    When I visit the kvs page for yaml
    ---
      dc: datacenter
      kv: foo/bar/[Action]/
    ---
    Then the url should be /datacenter/kv/foo/bar/[Action]/
    And I see 1 kv model
    # And the last GET request was made to "/v1/kv/foo/bar/[Action]/?keys&dc=datacenter&separator=%2F"
  Where:
    ----------
    | Action |
    | edit   |
    | create |
    ----------

@ignore
  Scenario: I have 1 key in the foo/bar/[Action]/ folder and I visit without a trailing slash
    Given 1 kv model from yaml
    ---
      - edit
    ---
    And the url "/v1/kv/foo/bar" responds with a 404 status
    When I visit the kvs page for yaml
    ---
      dc: datacenter
      kv: foo/bar/[Action]
    ---
    Then the url should be /datacenter/kv/foo/bar/[Action]/
    And I see 1 kv model
    # And the last GET request was made to "/v1/kv/foo/bar/[Action]/?keys&dc=datacenter&separator=%2F"
  Where:
    ----------
    | Action |
    | edit   |
    | create |
    ----------
