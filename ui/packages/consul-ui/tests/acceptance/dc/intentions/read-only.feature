@setupApplicationTest
Feature: dc / intentions / read-only
  Scenario: Viewing a readonly intention
    Given 1 datacenter model with the value "dc1"
    And 1 intention model from yaml:
    ---
      Meta:
        external-source: kubernetes
    ---
    When I visit the intention page for yaml
    ---
      dc: dc1
      intention: default:external-source:web:default:external-source:db
    ---
    Then I see the "[data-test-readonly]" element
