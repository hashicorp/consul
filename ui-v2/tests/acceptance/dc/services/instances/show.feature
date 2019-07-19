@setupApplicationTest
Feature: dc / services / instances / show: Show Service Instance
  Background:
    Given 1 datacenter model with the value "dc1"
    And 2 instance models from yaml
    ---
    - Service:
        ID: service-0-with-id
        Meta:
          external-source: consul
      Node:
        Node: node-0
    - Service:
        ID: service-0-with-id
        Tags: ['Tag1', 'Tag2']
        Meta:
          external-source: nomad
      Node:
        Node: another-node
      Checks:
        - Name: Service check
          ServiceID: service-0
          Output: Output of check
          Status: passing
        - Name: Service check
          ServiceID: service-0
          Output: Output of check
          Status: warning
        - Name: Service check
          ServiceID: service-0
          Output: Output of check
          Status: critical
        - Name: Node check
          ServiceID: ""
          Output: Output of check
          Status: passing
        - Name: Node check
          ServiceID: ""
          Output: Output of check
          Status: warning
        - Name: Node check
          ServiceID: ""
          Output: Output of check
          Status: critical
    ---
    And 1 proxy model from yaml
    ---
    - ServiceProxy:
        DestinationServiceName: service-1
        DestinationServiceID: ~
    ---
  Scenario: A Service instance has no Proxy
    When I visit the instance page for yaml
    ---
      dc: dc1
      service: service-0
      node: another-node
      id: service-0-with-id
    ---
    Then the url should be /dc1/services/service-0/another-node/service-0-with-id
    Then I don't see type on the proxy
    Then I see externalSource like "nomad"

    And I don't see upstreams on the tabs
    And I see serviceChecksIsSelected on the tabs
    And I see 3 of the serviceChecks object

    When I click nodeChecks on the tabs
    And I see nodeChecksIsSelected on the tabs
    And I see 3 of the nodeChecks object

    When I click tags on the tabs
    And I see tagsIsSelected on the tabs

    Then I see the text "Tag1" in "[data-test-tags] span:nth-child(1)"
    Then I see the text "Tag2" in "[data-test-tags] span:nth-child(2)"
  Scenario: A Service instance warns when deregistered whilst blocking
    Given settings from yaml
    ---
    consul:client:
      blocking: 1
      throttle: 200
    ---
    And a network latency of 100
    When I visit the instance page for yaml
    ---
      dc: dc1
      service: service-0
      node: node-0
      id: service-0-with-id
    ---
    Then the url should be /dc1/services/service-0/node-0/service-0-with-id
    And an external edit results in 0 instance models
    And pause until I see the text "deregistered" in "[data-notification]"
  @ignore
    Scenario: A Service Instance's proxy blocking query is closed when the instance is deregistered
    Then ok

