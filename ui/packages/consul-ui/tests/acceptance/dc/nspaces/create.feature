@setupApplicationTest
@onlyNamespaceable
Feature: dc / nspaces / create
  Scenario:
    Given 1 datacenter model with the value "datacenter"
    When I visit the nspace page for yaml
    ---
      dc: datacenter
    ---
    Then the url should be /datacenter/namespaces/create
    And the title should be "New Namespace - Consul"

  Scenario: Creating a role uses an inline form
    Given 1 datacenter model with the value "datacenter"
    When I visit the nspace page for yaml
    ---
      dc: datacenter
    ---
    And I click roles.create
    Then I see the "[data-test-role-creator]" element
    And I fill in the roles.form with yaml
    ---
      Name: my-role
      Description: My role description
    ---
    And I click cancel on the roles.form
    Then I don't see the "[data-test-role-creator]" element
    And the url should be /datacenter/namespaces/create

  Scenario: Test we can create a Namespace
    Given 1 datacenter model with the value "datacenter"
    When I visit the nspace page for yaml
    ---
      dc: datacenter
    ---
    Then I fill in with yaml
    ---
      Name: namespace-name
      Description: namespace description
    ---
    And I submit
    Then a PUT request was made to "/v1/namespace/namespace-name?dc=datacenter" from yaml
    ---
      body:
        Name: namespace-name
        Description: namespace description
    ---
    Then the url should be /datacenter/namespaces
    And "[data-notification]" has the "hds-toast" class
    And "[data-notification]" has the "hds-alert--color-success" class
