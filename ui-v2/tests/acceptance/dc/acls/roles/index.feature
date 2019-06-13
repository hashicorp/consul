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
  Scenario: Searching the roles
    Given 1 datacenter model with the value "dc-1"
    And 3 role models from yaml
    ---
    - Description: Description Search
      Policies:
        - Name: not-in-Polsearch
      ServiceIdentities:
        - ServiceName: not-in-sisearch
    - Description: Not in descsearch
      Policies:
        - Name: Policy-Search
      ServiceIdentities:
        - ServiceName: not-in-sisearch
    - Description: Not in descsearch either
      Policies:
        - Name: not-in-Polsearch-either
      ServiceIdentities:
        - ServiceName: Si-Search
    ---
    When I visit the roles page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/roles
    Then I see 3 role models
    Then I fill in with yaml
    ---
    s: Description
    ---
    And I see 1 role model
    And I see 1 role model with the description "Description Search"
    Then I fill in with yaml
    ---
    s: Policy-Search
    ---
    And I see 1 role model
    And I see 1 role model with the policy "Policy-Search"
    Then I fill in with yaml
    ---
    s: Si-Search
    ---
    And I see 1 role model
    And I see 1 role model with the serviceIdentity "Si-Search"
