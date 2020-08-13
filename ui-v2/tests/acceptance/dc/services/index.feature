@setupApplicationTest
Feature: dc / services / index: List Services
  Scenario: Viewing the service list page with services
    Given 1 datacenter model with the value "dc-1"
    And 10 service models from yaml
    ---
      - Name: Service-0
        ExternalSources:
          - consul
        Kind: ~
      - Name: Service-0-proxy
        Kind: 'connect-proxy'
      - Name: Service-1
        ExternalSources:
          - nomad
        Kind: ~
      - Name: Service-1-proxy
        Kind: 'connect-proxy'
      - Name: Service-2
        ExternalSources:
          - terraform
        Kind: ~
      - Name: Service-2-proxy
        Kind: 'connect-proxy'
      - Name: Service-3
        ExternalSources:
          - kubernetes
        Kind: ~
      - Name: Service-3-proxy
        Kind: 'connect-proxy'
      - Name: Service-4
        ExternalSources:
          - aws
        Kind: ~
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
  Scenario: Viewing the service list page with gateways
    Given 1 datacenter model with the value "dc-1"
    And 3 service models from yaml
    ---
      - Name: Service-0-proxy
        Kind: 'connect-proxy'
      - Name: Service-1-ingress-gateway
        Kind: 'ingress-gateway'
      - Name: Service-2-terminating-gateway
        Kind: 'terminating-gateway'
    ---

    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services
    And the title should be "Services - Consul"
    Then I see 2 service models
    And I see kind on the services like yaml
    ---
    - ingress-gateway
    - terminating-gateway
    ---
  Scenario: View a Service in mesh
    Given 1 datacenter model with the value "dc-1"
    And 3 service models from yaml
    ---
      - Name: Service-0
        Kind: ~
        ConnectedWithProxy: true
        ConnectedWithGateway: true
      - Name: Service-0-proxy
        Kind: connect-proxy
      - Name: Service-1
        Kind: ~
        ConnectedWithProxy: false
        ConnectedWithGateway: false
    ---

    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services
    And the title should be "Services - Consul"
    Then I see 2 service models
    And I see mesh on the services.0
    And I don't see mesh on the services.1
  Scenario: View a Service's Associated Service count
    Given 1 datacenter model with the value "dc-1"
    And 3 service models from yaml
    ---
      - Name: Service-0
        Kind: ~
      - Name: Service-0-proxy
        Kind: connect-proxy
      - Name: Service-1
        Kind: 'ingress-gateway'
        GatewayConfig:
        - AssociatedServiceCount: 345
    ---

    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services
    And the title should be "Services - Consul"
    Then I see 2 service models
    And I don't see associatedServiceCount on the services.0
    And I see associatedServiceCount on the services.1
