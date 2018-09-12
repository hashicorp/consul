@setupApplicationTest
Feature: dc / services: List Services
  Scenario:
    Given 1 datacenter model with the value "dc-1"
    And 5 service models from yaml
    ---
    - Name: Service 1
      Meta:
        external-source: consul
    - Name: Service 2
      Meta:
        external-source: nomad
    - Name: Service 3
      Meta:
        external-source: terraform
    - Name: Service 4
      Meta:
        external-source: kubernetes
    - Name: Service 5
      Meta: ~
    ---
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services
    Then I see 5 service models
    And I see externalSource on the services like yaml
    ---
    - consul
    - nomad
    - terraform
    - kubernetes
    - ~
    ---

