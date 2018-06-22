@setupApplicationTest
Feature: dc / services / show: Show Service
  Scenario: Given various service with various tags, all tags are displayed
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

