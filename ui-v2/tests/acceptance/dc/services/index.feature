@setupApplicationTest
Feature: dc / services / index: List Services
  Scenario:
    Given 1 datacenter model with the value "dc-1"
    And 6 service models from yaml
    ---
    - Name: Service 1
      ExternalSources:
        - consul
    - Name: Service 2
      ExternalSources:
        - nomad
    - Name: Service 3
      ExternalSources:
        - terraform
    - Name: Service 4
      ExternalSources:
        - kubernetes
    - Name: Service 5
      ExternalSources: []
    - Name: Service 6
      ExternalSources: ~
    ---
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services
    And the title should be "Services - Consul"
    Then I see 6 service models
    And I see externalSource on the services like yaml
    ---
    - consul
    - nomad
    - terraform
    - kubernetes
    - ~
    - ~
    ---

