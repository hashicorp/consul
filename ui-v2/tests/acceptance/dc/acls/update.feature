@setupApplicationTest
Feature: dc / acls / update: ACL Update
  Background:
    Given 1 datacenter model with the value "datacenter"
    And I'm using a legacy token
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
  Scenario: Update to [Name], [Type], [Rules]
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
    Then the url should be /datacenter/acls
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
    Where:
      ----------------------------------------------------------
      | Name       | Type       |  Rules                       |
      | key-name   | client     |  node "0" {policy = "read"}  |
      | key name   | management |  node "0" {policy = "write"} |
      | key%20name | client     |  node "0" {policy = "read"}  |
      | utf8?      | management |  node "0" {policy = "write"} |
      ----------------------------------------------------------
  Scenario: There was an error saving the key
    Given the url "/v1/acl/update" responds with a 500 status
    And I submit
    Then the url should be /datacenter/acls/key
    Then "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "error" class
# @ignore
  # Scenario: Rules can be edited/updated
  #   Then ok
