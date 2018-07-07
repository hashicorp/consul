@setupApplicationTest
Feature: dc / services / show: Show Service
  Scenario: Given various services with various tags, all tags are displayed
    Given 1 datacenter model with the value "dc1"
    And 3 node models
    And 1 service model from yaml
    ---
    - Service:
        Tags: ['Tag1', 'Tag2']
    - Service:
        Tags: ['Tag3', 'Tag1']
    - Service:
        Tags: ['Tag2', 'Tag3']
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service-0
    ---
    Then I see the text "Tag1" in "[data-test-tags] span:nth-child(1)"
    Then I see the text "Tag2" in "[data-test-tags] span:nth-child(2)"
    Then I see the text "Tag3" in "[data-test-tags] span:nth-child(3)"
  Scenario: Given various services the various ports on their nodes are displayed
    Given 1 datacenter model with the value "dc1"
    And 3 node models
    And 1 service model from yaml
    ---
    - Checks:
        - Status: passing
      Service:
        Port: 8080
      Node:
        Address: 1.1.1.1
    - Service:
        Port: 8000
      Node:
        Address: 2.2.2.2
    - Service:
        Port: 8888
      Node:
        Address: 3.3.3.3
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service-0
    ---
    Then I see address on the healthy like yaml
    ---
      - "1.1.1.1:8080"
    ---
    Then I see address on the unhealthy like yaml
    ---
      - "2.2.2.2:8000"
      - "3.3.3.3:8888"
    ---
