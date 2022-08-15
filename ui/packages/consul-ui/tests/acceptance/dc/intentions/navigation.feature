@setupApplicationTest
Feature: dc / intentions / navigation
  Background:
    Given 1 datacenter model with the value "dc-1"
    And 3 intention models from yaml
    ---
    - ID: 755b72bd-f5ab-4c92-90cc-bed0e7d8e9f0
      Action: allow
      Meta: ~
      SourcePeer: ""
    - ID: 755b72bd-f5ab-4c92-90cc-bed0e7d8e9f1
      Action: deny
      Meta: ~
    - ID: 0755b72bd-f5ab-4c92-90cc-bed0e7d8e9f2
      Action: deny
      Meta: ~
    ---
  Scenario: Clicking a intention in the listing and back again
    When I visit the intentions page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/intentions
    And the title should be "Intentions - Consul"
    Then I see 3 intention models on the intentionList component
    Given 1 intention model from yaml
    ---
    ID: 755b72bd-f5ab-4c92-90cc-bed0e7d8e9f0
    ---
    When I click intention on the intentionList.intentions component
    Then a GET request was made to "/v1/internal/ui/services?dc=dc-1&ns=*"
    And I click "[data-test-back]"
    Then the url should be /dc-1/intentions
  Scenario: Clicking the create button and back again
    When I visit the intentions page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/intentions
    And the title should be "Intentions - Consul"
    Then I see 3 intention models on the intentionList component
    When I click create
    Then the url should be /dc-1/intentions/create
    And I click "[data-test-back]"
    Then the url should be /dc-1/intentions
