@setupApplicationTest
Feature: dc / services / index: List Services
  Scenario:
    Given 1 datacenter model with the value "dc-1"
    And 10 service models from yaml
    ---
      - Name: Service-0
        ExternalSources:
          - consul
      - Name: Service-0-proxy
        Kind: 'connect-proxy'
      - Name: Service-1
        ExternalSources:
          - nomad
      - Name: Service-1-proxy
        Kind: 'connect-proxy'
      - Name: Service-2
        ExternalSources:
          - terraform
      - Name: Service-2-proxy
        Kind: 'connect-proxy'
      - Name: Service-3
        ExternalSources:
          - kubernetes
      - Name: Service-3-proxy
        Kind: 'connect-proxy'
      - Name: Service-4
        ExternalSources:
          - aws
      - Name: Service-4-proxy
        Kind: 'connect-proxy'
    ---

    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services
    And the title should be "Services - Consul"
    Then I see 5 service models
    And I see externalSource on the services like yaml
    ---
    - consul
    - nomad
    - terraform
    - kubernetes
    - aws
    ---

