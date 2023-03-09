@setupApplicationTest
Feature: dc / services / index: List Services
  Scenario: Viewing the service list page with services
    Given 1 datacenter model with the value "dc-1"
    And 10 service models from yaml
    ---
      - Name: Service-0
        ExternalSources:
          - consul
        ChecksPassing: 0
        ChecksWarning: 0
        ChecksCritical: 10
        Kind: ~
      - Name: Service-0-proxy
        Kind: 'connect-proxy'
      - Name: Service-1
        ExternalSources:
          - nomad
        ChecksPassing: 0
        ChecksWarning: 0
        ChecksCritical: 9
        Kind: ~
      - Name: Service-1-proxy
        Kind: 'connect-proxy'
      - Name: Service-2
        ExternalSources:
          - terraform
        ChecksPassing: 0
        ChecksWarning: 0
        ChecksCritical: 8
        Kind: ~
      - Name: Service-2-proxy
        Kind: 'connect-proxy'
      - Name: Service-3
        ExternalSources:
          - kubernetes
        ChecksPassing: 0
        ChecksWarning: 0
        ChecksCritical: 7
        Kind: ~
      - Name: Service-3-proxy
        Kind: 'connect-proxy'
      - Name: Service-4
        ExternalSources:
          - aws
        ChecksPassing: 0
        ChecksWarning: 0
        ChecksCritical: 6
        Kind: ~
      - Name: Service-4-proxy
        Kind: 'connect-proxy'
        ChecksPassing: 0
        ChecksWarning: 0
        ChecksCritical: 5
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
    And 4 service models from yaml
    ---
      - Name: Service-0-proxy
        Kind: 'connect-proxy'
        ChecksPassing: 0
        ChecksWarning: 0
        ChecksCritical: 3
      - Name: Service-1-ingress-gateway
        Kind: 'ingress-gateway'
        ChecksPassing: 0
        ChecksWarning: 0
        ChecksCritical: 2
      - Name: Service-2-terminating-gateway
        Kind: 'terminating-gateway'
        ChecksPassing: 0
        ChecksWarning: 0
        ChecksCritical: 1
      - Name: Service-3-api-gateway
        Kind: 'api-gateway'
        ChecksPassing: 0
        ChecksWarning: 0
        ChecksCritical: 1
    ---

    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services
    And the title should be "Services - Consul"
    Then I see 3 service models
    And I see kind on the services like yaml
    ---
    - ingress-gateway
    - terminating-gateway
    - api-gateway
    ---
  Scenario: View a Service in mesh
    Given 1 datacenter model with the value "dc-1"
    And 3 service models from yaml
    ---
      - Name: Service-0
        Kind: ~
        ConnectedWithProxy: true
        ConnectedWithGateway: true
        ChecksPassing: 0
        ChecksWarning: 0
        ChecksCritical: 2
      - Name: Service-0-proxy
        Kind: connect-proxy
      - Name: Service-1
        Kind: ~
        ConnectedWithProxy: false
        ConnectedWithGateway: false
        ChecksPassing: 0
        ChecksWarning: 0
        ChecksCritical: 1
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
        ChecksPassing: 0
        ChecksWarning: 0
        ChecksCritical: 2
      - Name: Service-0-proxy
        Kind: connect-proxy
        ChecksPassing: 0
        ChecksWarning: 0
        ChecksCritical: 1
      - Name: Service-1
        Kind: 'ingress-gateway'
        ChecksPassing: 0
        ChecksWarning: 0
        ChecksCritical: 1
        GatewayConfig:
          AssociatedServiceCount: 345
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
  Scenario: Viewing the services index page with no services and ACLs enabled 
    Given 1 datacenter model with the value "dc-1"
    And 0 service models
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services
    And the title should be "Services - Consul"
    Then I see 0 service models 
    And I see the text "There don't seem to be any registered services in this Consul cluster, or you may not have service:read and node:read access to this view. Use Terraform, Kubernetes CRDs, Vault, or the Consul CLI to register Services." in ".empty-state p"
    And I see the "[data-test-empty-state-login]" element
  Scenario: Viewing the services index page with no services and ACLs disabled
    Given ACLs are disabled
    Given 1 datacenter model with the value "dc-1"
    And 0 service models
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services
    And the title should be "Services - Consul"
    Then I see 0 service models 
    And I see the text "There don't seem to be any registered services in this Consul cluster." in ".empty-state p"
    And I don't see the "[data-test-empty-state-login]" element
