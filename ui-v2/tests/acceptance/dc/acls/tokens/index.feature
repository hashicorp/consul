@setupApplicationTest
Feature: dc / acls / tokens / index: ACL Token List

  Scenario:
    Given 1 datacenter model with the value "dc-1"
    And 3 token models
    When I visit the tokens page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/tokens
    And I click actions on the tokens
    Then I see 3 token models
