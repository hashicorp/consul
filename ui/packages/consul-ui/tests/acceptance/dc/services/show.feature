@setupApplicationTest
Feature: dc / services / show: Show Service
  Scenario: Given a service with an external source, the logo is displayed
    Given 1 datacenter model with the value "dc1"
    And 1 node models
    And 1 service model from yaml
    ---
    - Service:
        Kind: ~
        Tags: ['Tag1', 'Tag2']
        Meta:
          external-source: consul
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service-0
    ---
    Then I see externalSource like "consul"
    And the title should be "service-0 - Consul"

  Scenario: Given a service with an 'unsupported' external source, there is no logo
    Given 1 datacenter model with the value "dc1"
    And 1 node models
    And 1 service model from yaml
    ---
    - Service:
        Kind: ~
        Tags: ['Tag1', 'Tag2']
        Meta:
          external-source: 'not-supported'
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service-0
    ---
    Then I don't see externalSource
  Scenario: Given various services with various tags, all tags are displayed
    Given 1 datacenter model with the value "dc1"
    And 3 node models
    And 1 service model from yaml
    ---
    - Service:
        Kind: ~
        Tags: ['Tag1', 'Tag2']
    - Service:
        Kind: ~
        Tags: ['Tag3', 'Tag1']
    - Service:
        Kind: ~
        Tags: ['Tag2', 'Tag3']
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service-0
    ---
    And I click tags on the tabs
    Then I see the text "Tag1" in "[data-test-tags] span:nth-child(1)"
    Then I see the text "Tag2" in "[data-test-tags] span:nth-child(2)"
    Then I see the text "Tag3" in "[data-test-tags] span:nth-child(3)"
  Scenario: Given various services the various nodes on their instances are displayed
    Given 1 datacenter model with the value "dc1"
    And 3 node models
    And 1 service model from yaml
    ---
    - Checks:
        - Status: critical
      Service:
        Kind: ~
        ID: passing-service-8080
        Port: 8080
        Address: 1.1.1.1
      Node:
        Address: 1.2.2.2
    - Checks:
        - Status: warning
      Service:
        Kind: ~
        ID: service-8000
        Port: 8000
        Address: 2.2.2.2
      Node:
        Address: 2.3.3.3
    - Checks:
        - Status: passing
      Service:
        Kind: ~
        ID: service-8888
        Port: 8888
        Address: 3.3.3.3
      Node:
        Address: 3.4.4.4
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service-0
    ---
    And I click instances on the tabs
    Then I see address on the instances like yaml
    ---
      - "1.1.1.1:8080"
      - "2.2.2.2:8000"
      - "3.3.3.3:8888"
    ---
  Scenario: Given a combination of sources I should see them all on the instances 
    Given 1 datacenter model with the value "dc1"
    And 3 node models
    And 1 service model from yaml
    ---
    - Checks:
        - Status: critical
      Service:
        Kind: ~
        ID: passing-service-8080
        Port: 8080
        Address: 1.1.1.1
        Meta:
          external-source: kubernetes
      Node:
        Address: 1.2.2.2
        Meta:
          synthetic-node: false
    - Checks:
        - Status: passing
      Service:
        Kind: ~
        ID: service-8000
        Port: 8000
        Address: 2.2.2.2
        Meta:
          external-source: kubernetes
      Node:
        Address: 2.3.3.3
        Meta:
          synthetic-node: false
    - Checks:
        - Status: passing
      Service:
        Kind: ~
        ID: service-8888
        Port: 8888
        Address: 3.3.3.3
        Meta:
          external-source: vault
      Node:
        Address: 3.4.4.4
        Meta:
          synthetic-node: false
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service-0
    ---
    And I click instances on the tabs
    Then I see externalSource on the instances vertically like yaml
    ---
      - "kubernetes"
      - "kubernetes"
      - "vault"
    ---
    And I see nodeName on the instances like yaml
    ---
      - "node-0"
      - "node-1"
      - "node-2"
    ---
    And I see nodeChecks on the instances
  Scenario: Given instances share the same external source, only show it at the top
    Given 1 datacenter model with the value "dc1"
    And 3 node models
    And 1 service model from yaml
    ---
    - Checks:
        - Status: critical
      Service:
        Kind: ~
        ID: passing-service-8080
        Port: 8080
        Address: 1.1.1.1
        Meta:
          external-source: kubernetes
      Node:
        Address: 1.2.2.2
        Meta:
          synthetic-node: true
    - Checks:
        - Status: passing
      Service:
        Kind: ~
        ID: service-8000
        Port: 8000
        Address: 2.2.2.2
        Meta:
          external-source: kubernetes
      Node:
        Address: 2.3.3.3
        Meta:
          synthetic-node: true
    - Checks:
        - Status: passing
      Service:
        Kind: ~
        ID: service-8888
        Port: 8888
        Address: 3.3.3.3
        Meta:
          external-source: kubernetes
      Node:
        Address: 3.4.4.4
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service-0
    ---
    And I click instances on the tabs
    Then I see externalSource like "kubernetes"
    And I don't see externalSource on the instances
  Scenario: Given one agentless instance, it should not show node health checks or node name
    Given 1 datacenter model with the value "dc1"
    And 1 node models
    And 1 service model from yaml
    ---
    - Checks:
        - Status: critical
      Service:
        Kind: ~
        ID: passing-service-8080
        Port: 8080
        Address: 1.1.1.1
        Meta:
          external-source: kubernetes
      Node:
        Address: 1.2.2.2
        Meta:
          synthetic-node: true
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service-0
    ---
    And I click instances on the tabs
    And I don't see nodeChecks on the instances
    And I don't see nodeName on the instances
  Scenario: Given a dashboard template has been set
    Given 1 datacenter model with the value "dc1"
    And ui_config from yaml
    ---
    dashboard_url_templates:
      service: https://something.com?{{Service.Name}}&{{Datacenter}}
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service-0
    ---
    # The external dashboard link should use the Service.Name not the ID
    And I see href on the dashboardAnchor like "https://something.com?service-0&dc1"
  Scenario: With no access to service
    Given 1 datacenter model with the value "dc1"
    And permissions from yaml
    ---
    service:
      read: false
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service-0
    ---
    Then I see status on the error like "403"
  Scenario: When access is removed from a service
    Given 1 datacenter model with the value "dc1"
    And 1 node models
    And 1 service model from yaml
    And a network latency of 100
    And permissions from yaml
    ---
    service:
      read: true
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service-0
    ---
    And I click instances on the tabs
    And I see 1 instance model
    Given permissions from yaml
    ---
    service:
      read: false
    ---
    # authorization requests are not blocking so we just wait until the next
    # service blocking query responds
    Then pause until I see the text "no longer have access" in "[data-notification]"
    And "[data-notification]" has the "error" class
    And I see status on the error like "403"

