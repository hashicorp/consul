# TODO: If we keep separate types of catalog filters then
# these tests need splitting out, if we are moving nodes
# to use the name filter UI also, then they can stay together
@setupApplicationTest
Feature: components / catalog-filter
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
    When I click serviceInstances on the tabs
    And I see serviceInstancesIsSelected on the tabs

    Then I fill in with yaml
    ---
    s: 65535
    ---
    And I see 1 [Model] model
    And I see 1 [Model] model with the port "65535"
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
      - Name: Service-0
        Tags: ['one', 'two', 'three']
        ChecksPassing: 0
        ChecksWarning: 0
        ChecksCritical: 1
        Kind: ~
      - Name: Service-1
        Tags: ['two', 'three']
        ChecksPassing: 0
        ChecksWarning: 1
        ChecksCritical: 0
        Kind: ~
      - Name: Service-2
        Tags: ['three']
        ChecksPassing: 1
        ChecksWarning: 0
        ChecksCritical: 0
        Kind: ~


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