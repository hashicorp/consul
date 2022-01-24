@setupApplicationTest
Feature: dc / routing-config
  Scenario: Viewing a routing config
    Given 1 datacenter model with the value "dc1"
    When I visit the routingConfig page for yaml
    ---
      dc: dc1
      name: virtual-1
    ---
    Then the url should be /dc1/routing-config/virtual-1
    Then I don't see status on the error like "404"
    And the title should be "virtual-1 - Consul"
  Scenario: Viewing a source pill
    Given 1 datacenter model with the value "dc1"
    When I visit the routingConfig page for yaml
    ---
      dc: dc1
      name: virtual-1
    ---
    And I see source

