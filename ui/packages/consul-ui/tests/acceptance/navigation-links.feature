@setupApplicationTest
Feature: navigation-links: Main Navigation link visibility
  Scenario: No read access to Key/Values
    Given 1 datacenter model with the value "dc-1"
    And the url "/v1/internal/acl/authorize" responds with from yaml
    ---
    body:
      - Resource: operator
        Access: write
        Allow: true
      - Resource: service
        Access: read
        Allow: true
      - Resource: node
        Access: read
        Allow: true
      - Resource: key
        Access: read
        Allow: true
      - Resource: intention
        Access: read
        Allow: true
      - Resource: acl
        Access: read
        Allow: false
    ---
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then I see services on the navigation
    Then I don't see roles on the navigation

  Scenario: Empty state login button is shown
    Given 1 datacenter model with the value "dc-1"
    And 0 service models
    When I visit the services page for yaml
    ---
    dc: dc-1
    ---
    Then the url should be /dc-1/services
    And I see login on the emptystate
