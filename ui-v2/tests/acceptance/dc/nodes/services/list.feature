@setupApplicationTest
Feature: dc / nodes / services / list: Node > Services Listing
  Scenario: Given 1 node
    Given 1 datacenter model with the value "dc1"
    And 1 node model from yaml
    ---
    ID: node-0
    Services:
    - ID: 'service-0-with-id'
      Port: 65535
      Service: 'service-0'
      Tags: ['monitor', 'two', 'three']
      Meta:
        external-source: consul
    - ID: 'service-1'
      Port: 0
      Service: 'service-1'
      Tags: ['hard drive', 'monitor', 'three']
      Meta:
        external-source: nomad
    - ID: 'service-2'
      Port: 1
      Service: 'service-2'
      Tags: ['one', 'two', 'three']
      Meta:
        external-source: terraform
    - ID: 'service-3'
      Port: 2
      Service: 'service-3'
      Tags: []
      Meta:
        external-source: kubernetes
    - ID: 'service-4'
      Port: 3
      Service: 'service-4'
      Tags: []
      Meta: ~
    ---
    When I visit the node page for yaml
    ---
      dc: dc1
      node: node-0
    ---
    When I click services on the tabs
    And I see servicesIsSelected on the tabs
    And I see externalSource on the services like yaml
    ---
    - consul
    - nomad
    - terraform
    - kubernetes
    - ~
    ---
