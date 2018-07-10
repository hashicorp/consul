@setupApplicationTest
Feature: components / catalog-filter
  Scenario: Filtering [Model]
    Given 1 datacenter model with the value "dc-1"
    And 3 service models from yaml
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
    ---
    And 3 node models from yaml
    ---
      - Checks:
          - Status: passing
      - Checks:
          - Status: warning
      - Checks:
          - Status: critical

    ---
    When I visit the [Page] page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be [Url]

    Then I see 3 [Model] models
    And I see allIsSelected on the filter

    When I click passing on the filter
    And I see passingIsSelected on the filter
    And I see 1 [Model] model

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
    | service | services | /dc-1/services            |
    | node    | nodes    | /dc-1/nodes               |
    -------------------------------------------------
  Scenario: Filtering [Model] in [Page]
    Given 1 datacenter model with the value "dc1"
    And 2 node models from yaml
    ---
    - ID: node-0
    ---
    When I visit the node page for yaml
    ---
      dc: dc1
      node: node-0
    ---
    # And I see 3 healthcheck model with the name "Disk Util"
    # And then pause for 5000
    When I click services on the tabs
    And I see servicesIsSelected on the tabs

    Then I fill in with yaml
    ---
    s: 65535
    ---
    And I see 1 [Model] model
    And I see 1 [Model] model with the port "65535"
  Where:
    -------------------------------------------------
    | Model   | Page     | Url                       |
    | service | node     | /dc-1/nodes/node-0        |
    -------------------------------------------------
