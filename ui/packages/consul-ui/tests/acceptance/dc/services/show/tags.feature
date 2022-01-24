@setupApplicationTest
Feature: dc / services / show / tags
  Background:
    Given 1 datacenter model with the value "dc1"
    And 1 node models
  Scenario: A service with multiple tags
    Given 1 service model from yaml
    ---
    - Service:
        Name: service
        Kind: ~
        Tags:
          - tag
          - tag1
          - tag2
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service
    ---
    And I see tags on the tabs
    When I click tags on the tabs
    And I see tagsIsSelected on the tabs
    And I see 3 tag models on the tabs.tagsTab component
  Scenario: A service with multiple duplicated tags
    Given 1 service model from yaml
    ---
    - Service:
        Name: service
        Kind: ~
        Tags:
          - tag1
          - tag2
          - tag
          - tag
          - tag1
          - tag2
    ---
    When I visit the service page for yaml
    ---
      dc: dc1
      service: service
    ---
    And I see tags on the tabs
    When I click tags on the tabs
    And I see tagsIsSelected on the tabs
    And I see 3 tag models on the tabs.tagsTab component
