@setupApplicationTest
Feature: dc / acls / delete: ACL Delete
  Scenario: Delete ACL
    Given 1 datacenter model with the value "datacenter"
    And 1 acl model from yaml
    ---
      Name: something
      ID: key
    ---
    When I visit the acls page for yaml
    ---
      dc: datacenter
    ---
    And I click actions on the acls
    And I click delete on the acls
    And I click confirmDelete on the acls
    Then a PUT request is made to "/v1/acl/destroy/key?dc=datacenter"
