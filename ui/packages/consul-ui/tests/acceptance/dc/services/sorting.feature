@setupApplicationTest
Feature: dc / services / sorting
  Scenario: Sorting the service list by name
    Given 1 datacenter model with the value "dc-1"
    And 4 service models from yaml
    ---
    - Name: Service-B
      Kind: ~
    - Name: Service-D
      Kind: ~
    - Name: Service-A
      Kind: ~
    - Name: Service-C
      Kind: ~
    ---
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    # ascending (A-Z)
    When I click name on the sort
    Then I see name on the services vertically like yaml
    ---
    - Service-A
    - Service-B
    - Service-C
    - Service-D
    ---
    # descending (Z-A)
    When I click name on the sort
    Then I see name on the services vertically like yaml
    ---
    - Service-D
    - Service-C
    - Service-B
    - Service-A
    ---
