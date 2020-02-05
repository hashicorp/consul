@setupApplicationTest
Feature: dc / acls / policies / create
  Scenario: 
    Given 1 datacenter model with the value "datacenter"
    When I visit the policy page for yaml
    ---
      dc: datacenter
    ---
    Then the url should be /datacenter/acls/policies/create
    And the title should be "New Policy - Consul"

@ignore
  Scenario: Test we can create a ACLs Policy
  Then ok
