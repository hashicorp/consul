@setupApplicationTest
Feature: dc / acls / roles / index: ACL Roles List

  Scenario:
    Given 1 datacenter model with the value "dc-1"
    And 3 role models
    When I visit the roles page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/roles
    Then I see 3 role models
