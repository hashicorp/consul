@setupApplicationTest
@onlyNamespaceable
Feature: dc / nspaces / update: Nspace Update
  Background:
    Given 1 datacenter model with the value "datacenter"
    And 1 nspace model from yaml
    ---
      Name: namespace
      Description: empty
      PolicyDefaults: ~
    ---
    When I visit the nspace page for yaml
    ---
      dc: datacenter
      namespace: namespace
    ---
    Then the url should be /datacenter/namespaces/namespace
    And the title should be "Edit namespace - Consul"
  Scenario: Update to [Description]
    Then I fill in with yaml
    ---
      Description: [Description]
    ---
    And I submit
    Then a PUT request was made to "/v1/namespace/namespace?dc=datacenter" from yaml
    ---
      body:
        Description: [Description]
    ---
    Then the url should be /datacenter/namespaces
    And "[data-notification]" has the "hds-toast" class
    And "[data-notification]" has the "hds-alert--color-success" class
    Where:
      ---------------------------
      | Description             |
      | description             |
      | description with spaces |
      ---------------------------
  Scenario: There was an error saving the key
    Given the url "/v1/namespace/namespace?dc=datacenter" responds with a 500 status
    And I submit
    Then the url should be /datacenter/namespaces/namespace
    And "[data-notification]" has the "hds-toast" class
    And "[data-notification]" has the "hds-alert--color-critical" class
