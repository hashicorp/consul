@setupApplicationTest
Feature: dc / acls / delete: ACL Delete
  Scenario: Deleting an ACL from the ACL listing page
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
  Scenario: Deleting an ACL from the ACL detail page
    Given 1 datacenter model with the value "datacenter"
    And 1 acl model from yaml
    ---
      Name: something
      ID: key
    ---
    When I visit the acl page for yaml
    ---
      dc: datacenter
      acl: something
    ---
    And I click delete
    And I click confirmDelete
    Then a PUT request is made to "/v1/acl/destroy/something?dc=datacenter"
