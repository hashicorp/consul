@setupApplicationTest
Feature: components / text-input: Text input
  Background:
    Given 1 datacenter model with the value "dc-1"
  Scenario:
    When I visit the kv page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/kv/create
    # Turn the Code Editor off so we can fill the value easier
    And I click "[name=json]"
    Then I fill in with json
    ---
    {"additional": "hi", "value": "there"}
    ---
    Then I see submitIsEnabled
  Scenario:
    When I visit the acl page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/create
    Then I fill in with json
    ---
    {"name": "hi"}
    ---
    Then I see submitIsEnabled
