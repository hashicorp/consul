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
        ID: passing-service-8080
        Port: 8080
        Address: 1.1.1.1
      Node:
        Address: 1.2.2.2
    - Service:
        ID: service-8000
        Port: 8000
        Address: 2.2.2.2
      Node:
        Address: 2.3.3.3
    - Service:
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
    Then I see address on the healthy like yaml
    ---
      - "1.1.1.1:8080"
    ---
    Then I see address on the unhealthy like yaml
    ---
      - "2.2.2.2:8000"
      - "3.3.3.3:8888"
    ---
    Then I see id on the healthy like yaml
    ---
      - "passing-service-8080"
    ---
    Then I see id on the unhealthy like yaml
    ---
      - "service-8000"
      - "service-8888"
    ---
