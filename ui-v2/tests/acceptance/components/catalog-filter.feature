# TODO: If we keep separate types of catalog filters then
# these tests need splitting out, if we are moving nodes
# to use the name filter UI also, then they can stay together
@setupApplicationTest
Feature: components / catalog-filter
  Scenario: Filtering [Model]
    Given 1 datacenter model with the value "dc-1"
    And 4 service models from yaml
    ---
      - ChecksPassing: 1
        ChecksWarning: 0
        ChecksCritical: 0
      - ChecksPassing: 0
        ChecksWarning: 1
        ChecksCritical: 0
      - ChecksPassing: 0
        ChecksWarning: 0
        ChecksCritical: 1
      - ChecksPassing: 1
        ChecksWarning: 0
        ChecksCritical: 0
    ---
    And 4 node models from yaml
    ---
      - Checks:
          - Status: passing
      - Checks:
          - Status: warning
      - Checks:
          - Status: critical
      - Checks:
          - Status: passing
    ---
    When I visit the [Page] page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be [Url]

    Then I see 4 [Model] models
    And I see allIsSelected on the filter

    When I click passing on the filter
    And I see passingIsSelected on the filter
    And I see 2 [Model] models

    When I click warning on the filter
    And I see warningIsSelected on the filter
    And I see 1 [Model] model

    When I click critical on the filter
    And I see criticalIsSelected on the filter
    And I see 1 [Model] model

    When I click all on the filter
    And I see allIsSelected on the filter
    Then I fill in with yaml
    ---
    s: [Model]-0
    ---
    And I see 1 [Model] model with the name "[Model]-0"

  Where:
    -------------------------------------------------
    | Model   | Page     | Url                       |
    | node    | nodes    | /dc-1/nodes               |
    -------------------------------------------------
  Scenario: Filtering [Model] in [Page]
    Given 1 datacenter model with the value "dc1"
    And 1 node model from yaml
    ---
    ID: node-0
    Services:
    - ID: 'service-0-with-id'
      Port: 65535
      Service: 'service-0'
      Tags: ['monitor', 'two', 'three']
    - ID: 'service-1'
      Port: 0
      Service: 'service-1'
      Tags: ['hard drive', 'monitor', 'three']
    - ID: 'service-2'
      Port: 1
      Service: 'service-2'
      Tags: ['one', 'two', 'three']
    ---
    When I visit the [Page] page for yaml
    ---
      dc: dc1
      node: node-0
    ---
    # And I see 3 healthcheck model with the name "Disk Util"
    When I click services on the tabs
    And I see servicesIsSelected on the tabs

    Then I fill in with yaml
    ---
    s: 65535
    ---
    And I see 1 [Model] model
    And I see 1 [Model] model with the port "65535"
    Then I fill in with yaml
    ---
    s: service-0-with-id
    ---
    And I see 1 [Model] model
    And I see 1 [Model] model with the id "service-0-with-id"
    Then I fill in with yaml
    ---
    s: hard drive
    ---
    And I see 1 [Model] model with the name "[Model]-1"
    Then I fill in with yaml
    ---
    s: monitor
    ---
    And I see 2 [Model] models
    Then I fill in with yaml
    ---
    s: wallpix
    ---
    And I see 0 [Model] models
  Where:
    -------------------------------------------------
    | Model   | Page     | Url                       |
    | service | node     | /dc-1/nodes/node-0        |
    -------------------------------------------------
  Scenario: Freetext filtering the service listing
    Given 1 datacenter model with the value "dc-1"
    And 3 service models from yaml
    ---
      - Tags: ['one', 'two', 'three']
        ChecksPassing: 0
        ChecksWarning: 0
        ChecksCritical: 1
      - Tags: ['two', 'three']
        ChecksPassing: 0
        ChecksWarning: 1
        ChecksCritical: 0
      - Tags: ['three']
        ChecksPassing: 1
        ChecksWarning: 0
        ChecksCritical: 0
    ---
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services
    Then I see 3 service models
    Then I fill in with yaml
    ---
    s: three
    ---
    And I see 3 service models
    Then I fill in with yaml
    ---
    s: 'tag:two'
    ---
    And I see 2 service models
    Then I fill in with yaml
    ---
    s: 'status:critical'
    ---
    And I see 1 service model
