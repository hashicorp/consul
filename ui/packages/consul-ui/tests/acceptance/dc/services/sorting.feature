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
  Scenario: Sorting the service list by health
    Given 1 datacenter model with the value "dc-1"
    And 3 service models from yaml
    ---
    - Name: Service-passing
      Kind: ~
      ChecksPassing: 1
      ChecksWarning: 0
      ChecksCritical: 0
    - Name: Service-critical
      Kind: ~
      ChecksPassing: 0
      ChecksWarning: 0
      ChecksCritical: 1
    - Name: Service-warning
      Kind: ~
      ChecksPassing: 0
      ChecksWarning: 1
      ChecksCritical: 0
    ---
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    # ascending (unhealthy first)
    When I click health on the sort
    Then I see name on the services vertically like yaml
    ---
    - Service-critical
    - Service-warning
    - Service-passing
    ---
    # descending (healthy first)
    When I click health on the sort
    Then I see name on the services vertically like yaml
    ---
    - Service-passing
    - Service-warning
    - Service-critical
    ---
