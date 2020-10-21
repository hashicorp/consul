@setupApplicationTest
Feature: dc / intentions / index
  Scenario: Viewing intentions in the listing
    Given 1 datacenter model with the value "dc-1"
    And 3 intention models
    When I visit the intentions page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/intentions
    And the title should be "Intentions - Consul"
    Then I see 3 intention models
