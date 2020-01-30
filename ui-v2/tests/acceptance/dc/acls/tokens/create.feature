@setupApplicationTest
Feature: dc / acls / tokens / create
  Scenario: 
    Given 1 datacenter model with the value "datacenter"
    When I visit the token page for yaml
    ---
      dc: datacenter
    ---
    Then the url should be /datacenter/acls/tokens/create
    And the title should be "New Token - Consul"

@ignore
  Scenario: Test we can create a ACLs Token
  Then ok
