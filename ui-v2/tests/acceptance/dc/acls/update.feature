@setupApplicationTest
Feature: dc / acls / update: ACL Update
  Scenario: Update to [Name], [Type], [Rules]
    Given 1 datacenter model with the value "datacenter"
    And 1 acl model from yaml
    ---
      ID: key
    ---
    When I visit the acl page for yaml
    ---
      dc: datacenter
      acl: key
    ---
    Then the url should be /datacenter/acls/key
    Then I fill in with yaml
    ---
      name: [Name]
    ---
    And I click "[value=[Type]]"
    And I submit
    Then a PUT request is made to "/v1/acl/update?dc=datacenter" with the body from yaml
    ---
      Name: [Name]
      Type: [Type]
    ---
    Where:
      ----------------------------------------------------------
      | Name       | Type       |  Rules                       |
      | key-name   | client     |  node "0" {policy = "read"}  |
      | key name   | management |  node "0" {policy = "write"} |
      | key%20name | client     |  node "0" {policy = "read"}  |
      | utf8?      | management |  node "0" {policy = "write"} |
      ----------------------------------------------------------
@ignore
  Scenario: Rules can be edited/updated
    Then ok
@ignore
  Scenario: The feedback dialog says success or failure
    Then ok
