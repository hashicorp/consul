@setupApplicationTest
# FIXME
@ignore
Feature: dc / services / error
  Scenario: Arriving at the service page that doesn't exist
    Given 2 datacenter models from yaml
    ---
    - dc-1
    - dc-2
    ---
    When I visit the services page for yaml
    ---
      dc: 404-datacenter
    ---
    Then I see status on the error like "404"
  @notNamespaceable
  Scenario: Arriving at the service page
    Given 2 datacenter models from yaml
    ---
    - dc-1
    - dc-2
    ---
    Given the url "/v1/internal/ui/services" responds with a 500 status
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then I see status on the error like "500"
    # This is the actual step that works slightly differently
    # When running through namespaces as the dc menu says 'Error'
    # which is still kind of ok
    When I click dc on the navigation
    And I see 2 datacenter models on the navigation component
