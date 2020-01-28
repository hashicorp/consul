@setupApplicationTest
Feature: dc / acls / roles / create
  Scenario: 
    Given 1 datacenter model with the value "datacenter"
    When I visit the role page for yaml
    ---
      dc: datacenter
    ---
    Then the url should be /datacenter/acls/roles/create
    And the title should be "New Role - Consul"

@ignore
  Scenario: Test we can create a ACLs role
  Then ok
