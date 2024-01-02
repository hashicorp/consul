@setupApplicationTest
Feature: dc / services / instances / navigation
  Background:
    Given 1 datacenter model with the value "dc-1"
    And 1 proxy model from yaml
    ---
    ServiceName: service-0-proxy
    Node: node-0
    ServiceID: service-a-proxy
    ---
    And 3 instance models from yaml
    ---
    - Service:
        Service: service-0
        ID: service-a
      Node:
        Node: node-0
      Checks:
      - Status: critical
    - Service:
        Service: service-0
        ID: service-b
      Node:
        Node: node-0
      Checks:
      - Status: passing
    # A listing of instances from 2 services would never happen in consul but
    # this satisfies our mocking needs for the moment, until we have a 'And 1
    # proxy on request.0 from yaml', 'And 1 proxy on request.1 from yaml' or
    # similar
    - Service:
        Service: service-0-proxy
        ID: service-a-proxy
      Node:
        Node: node-0
      Checks:
      - Status: passing
    ---
