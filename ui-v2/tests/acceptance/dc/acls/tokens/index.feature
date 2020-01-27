@setupApplicationTest
Feature: dc / acls / tokens / index: ACL Token List

  Scenario: I see the tokens
    Given 1 datacenter model with the value "dc-1"
    And 3 token models
    When I visit the tokens page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/tokens
    Then I see 3 token models
  Scenario: Searching the tokens
    Given 1 datacenter model with the value "dc-1"
    And 4 token models from yaml
    ---
    - Description: Description Search
      Legacy: false
      ServiceIdentities:
        - ServiceName: not-in-sisearch
      Policies:
        - Name: not-in-Polsearch
      Roles:
        - Name: not-in-rolesearch
    - Description: Not in descsearch
      Legacy: false
      ServiceIdentities:
        - ServiceName: not-in-sisearch
      Policies:
        - Name: Policy-Search
      Roles:
        - Name: not-in-rolesearch-either
    - Description: Not in descsearch either
      Legacy: false
      ServiceIdentities:
        - ServiceName: not-in-sisearch
      Policies:
        - Name: not-int-Polsearch-either
      Roles:
        - Name: Role-Search
    - Description: Not in descsearch either
      Legacy: false
      ServiceIdentities:
        - ServiceName: Si-Search
      Policies:
        - Name: not-int-Polsearch-either
      Roles:
        - Name: not-in-rolesearch-either
    ---
    When I visit the tokens page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/tokens
    Then I see 4 token models
    Then I fill in with yaml
    ---
    s: Description
    ---
    And I see 1 token model
    And I see 1 token model with the description "Description Search"
    Then I fill in with yaml
    ---
    s: Policy-Search
    ---
    And I see 1 token model
    And I see 1 token model with the policy "Policy-Search"
    Then I fill in with yaml
    ---
    s: Role-Search
    ---
    And I see 1 token model
    And I see 1 token model with the role "Role-Search"
    Then I fill in with yaml
    ---
    s: Si-Search
    ---
    And I see 1 token model
    And I see 1 token model with the serviceIdentity "Si-Search"
  Scenario: I see the legacy message if I have one legacy token
    Given 1 datacenter model with the value "dc-1"
    And 3 token models from yaml
    ---
    - Legacy: true
    - Legacy: false
    - Legacy: false
    ---
    When I visit the tokens page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/tokens
    And I see update
    And I see 3 token models
  Scenario: I don't see the legacy message if I have no legacy tokens
    Given 1 datacenter model with the value "dc-1"
    And 3 token models from yaml
    ---
    - Legacy: false
    - Legacy: false
    - Legacy: false
    ---
    When I visit the tokens page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/tokens
    And I don't see update
    And I see 3 token models
