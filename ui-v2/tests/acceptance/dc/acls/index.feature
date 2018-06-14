@setupApplicationTest
Feature: dc / acls / index: ACL List

  Scenario:
    Given 1 datacenter model with the value "dc-1"
    And 3 acl models
    When I visit the acls page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls
    And I click actions on the acls
    Then I don't see delete on the acls
    Then I see 3 acl models
