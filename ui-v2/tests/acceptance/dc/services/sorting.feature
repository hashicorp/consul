@setupApplicationTest
Feature: dc / services / sorting
  Scenario:
    Given 1 datacenter model with the value "dc-1"
    And 6 service models from yaml
    ---
    - Name: Service-A
      Kind: ~
      ChecksPassing: 1
      ChecksWarning: 1
      ChecksCritical: 3
    - Name: Service-B
      Kind: ~
      ChecksPassing: 1
      ChecksWarning: 1
      ChecksCritical: 5
    - Name: Service-C
      Kind: ~
      ChecksPassing: 1
      ChecksWarning: 1
      ChecksCritical: 4
    - Name: Service-D
      Kind: ~
      ChecksPassing: 1
      ChecksWarning: 5
      ChecksCritical: 1
    - Name: Service-E
      Kind: ~
      ChecksPassing: 1
      ChecksWarning: 3
      ChecksCritical: 1
    - Name: Service-F
      Kind: ~
      ChecksPassing: 1
      ChecksWarning: 4
      ChecksCritical: 1
    ---
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    When I click selected on the sort
    # unhealthy / healthy
    When I click options.0.button on the sort
    Then I see name on the services vertically like yaml
    ---
    - Service-B
    - Service-C
    - Service-A
    - Service-D
    - Service-F
    - Service-E
    ---
    When I click selected on the sort
    # healthy / unhealthy
    When I click options.1.button on the sort
    Then I see name on the services vertically like yaml
    ---
    - Service-E
    - Service-F
    - Service-D
    - Service-A
    - Service-C
    - Service-B
    ---
    When I click selected on the sort
    When I click options.2.button on the sort
    Then I see name on the services vertically like yaml
    ---
    - Service-A
    - Service-B
    - Service-C
    - Service-D
    - Service-E
    - Service-F
    ---
    When I click selected on the sort
    When I click options.3.button on the sort
    Then I see name on the services vertically like yaml
    ---
    - Service-F
    - Service-E
    - Service-D
    - Service-C
    - Service-B
    - Service-A
    ---
