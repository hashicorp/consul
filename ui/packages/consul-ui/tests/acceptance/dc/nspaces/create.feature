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

@ignore
  Scenario: Test we can create a Namespace
  Then ok
