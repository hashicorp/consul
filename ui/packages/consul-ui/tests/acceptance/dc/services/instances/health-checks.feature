@setupApplicationTest
Feature: dc / services / instances / health-checks
  Background:
    Given 1 datacenter model with the value "dc1"
    And 1 proxy model from yaml
    ---
    - ServiceProxy:
        DestinationServiceName: service-1
        DestinationServiceID: ~
    ---
  Scenario: A failing serf check
    Given 2 instance models from yaml
    ---
    - Service:
        ID: service-0-with-id
      Node:
        Node: node-0
    - Service:
        ID: service-1-with-id
      Node:
        Node: another-node
      Checks:
        - Type: ''
          Name: Serf Health Status
          CheckID: serfHealth
          Status: critical
          Output: ouch
    ---
    When I visit the instance page for yaml
    ---
      dc: dc1
      service: service-0
      node: another-node
      id: service-1-with-id
    ---
    Then the url should be /dc1/services/service-0/instances/another-node/service-1-with-id/health-checks
    And I see healthChecksIsSelected on the tabs
    And I see criticalSerfNotice on the tabs.healthChecksTab
  Scenario: A passing serf check
    Given 2 instance models from yaml
    ---
    - Service:
        ID: service-0-with-id
      Node:
        Node: node-0
    - Service:
        ID: service-1-with-id
      Node:
        Node: another-node
      Checks:
        - Type: ''
          Name: Serf Health Status
          CheckID: serfHealth
          Status: passing
          Output: Agent alive and reachable
    ---
    When I visit the instance page for yaml
    ---
      dc: dc1
      service: service-0
      node: another-node
      id: service-1-with-id
    ---
    Then the url should be /dc1/services/service-0/instances/another-node/service-1-with-id/health-checks
    And I see healthChecksIsSelected on the tabs
    And I don't see criticalSerfNotice on the tabs.healthChecksTab
  Scenario: Node health check should be hidden on agentless service instances
    Given 1 instance model from yaml
    ---
    - Service:
        ID: service-0-with-id
      Node:
        Node: another-node
        Meta:
          synthetic-node: true 
      Checks:
        - Type: ''
          Name: Node Health Check
          CheckID: serfHealth
          ServiceID: ''
          Status: passing
          Output: Agent alive and reachable
        - Type: ''
          Name: Serf Health Status
          CheckID: serfHealth
          Status: critical
          Output: ouch
    ---
    When I visit the instance page for yaml
    ---
      dc: dc1
      service: service-0
      node: another-node
      id: service-0-with-id
    ---
    Then the url should be /dc1/services/service-0/instances/another-node/service-0-with-id/health-checks
    And I see healthChecksIsSelected on the tabs
    And I see 1 healthCheck model on the tabs.healthChecksTab component
    And I see 1 healthCheck model with the name "Serf Health Status"
  Scenario: Node health checks should be visible on non-agentless service instances
    Given 1 instance model from yaml
    ---
    - Service:
        ID: service-0-with-id
      Node:
        Node: another-node
      Checks:
        - Type: ''
          Name: Node Health Check
          CheckID: serfHealth
          ServiceID: ''
          Status: passing
          Output: Agent alive and reachable
        - Type: ''
          Name: Serf Health Status
          CheckID: serfHealth
          Status: critical
          Output: ouch
    ---
    When I visit the instance page for yaml
    ---
      dc: dc1
      service: service-0
      node: another-node
      id: service-0-with-id
    ---
    Then the url should be /dc1/services/service-0/instances/another-node/service-0-with-id/health-checks
    And I see healthChecksIsSelected on the tabs
    And I see 2 healthCheck models on the tabs.healthChecksTab component
    And I see 1 healthCheck model with the name "Serf Health Status"
    And I see 1 healthCheck model with the name "Node Health Check"
