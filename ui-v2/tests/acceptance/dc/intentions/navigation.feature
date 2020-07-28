@setupApplicationTest
Feature: dc / intentions / navigation
  Scenario: Clicking a intention in the listing and back again
    Given 1 datacenter model with the value "dc-1"
    And 3 intention models
    When I visit the intentions page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/intentions
    And the title should be "Intentions - Consul"
    Then I see 3 intention models
    When I click intention on the intentions
    Then a GET request was made to "/v1/internal/ui/services?dc=dc-1&ns=*"
    And I click "[data-test-back]"
    Then the url should be /dc-1/intentions
  Scenario: Clicking the create button and back again
    Given 1 datacenter model with the value "dc-1"
    And 3 intention models
    When I visit the intentions page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/intentions
    And the title should be "Intentions - Consul"
    Then I see 3 intention models
    When I click create
    Then the url should be /dc-1/intentions/create
    And I click "[data-test-back]"
    Then the url should be /dc-1/intentions
