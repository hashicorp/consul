@setupApplicationTest
Feature: Page Navigation
  Background:
    Given 1 datacenter model with the value "dc-1"
  Scenario: Visiting the index page
    When I visit the index page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services
    Then the last GET request was made to "/v1/internal/ui/services?dc=dc-1"
  Scenario: Clicking [Link] in the navigation takes me to [URL]
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    When I click [Link] on the navigation
    Then the url should be [URL]
    Then the last GET request was made to "[Endpoint]"
  Where:
    ----------------------------------------------------------------------
    | Link       | URL              | Endpoint                           |
    | nodes      | /dc-1/nodes      | /v1/internal/ui/nodes?dc=dc-1      |
    | kvs        | /dc-1/kv         | /v1/kv/?keys&dc=dc-1&separator=%2F |
    | acls       | /dc-1/acls       | /v1/acl/list?dc=dc-1               |
    | intentions | /dc-1/intentions | /v1/connect/intentions?dc=dc-1     |
    | settings   | /settings        | /v1/catalog/datacenters            |
    ----------------------------------------------------------------------
  Scenario: Clicking a [Item] in the [Model] listing and back again
    When I visit the [Model] page for yaml
    ---
      dc: dc-1
    ---
    When I click [Item] on the [Model]
    Then the url should be [URL]
    Then the last GET request was made to "[Endpoint]"
    And I click "[data-test-back]"
    Then the url should be [Back]
  Where:
    ------------------------------------------------------------------------------------------------------------------------------------------------------------------------
    | Item      | Model      | URL                                                      | Endpoint                                                      | Back             |
    | service   | services   | /dc-1/services/service-0                                 | /v1/health/service/service-0?dc=dc-1                          | /dc-1/services   |
    | node      | nodes      | /dc-1/nodes/node-0                                       | /v1/session/node/node-0?dc=dc-1                               | /dc-1/nodes      |
    | kv        | kvs        | /dc-1/kv/necessitatibus-0/edit                           | /v1/session/info/ee52203d-989f-4f7a-ab5a-2bef004164ca?dc=dc-1 | /dc-1/kv         |
    | acl       | acls       | /dc-1/acls/anonymous                                     | /v1/acl/info/anonymous?dc=dc-1                                | /dc-1/acls       |
    | intention | intentions | /dc-1/intentions/ee52203d-989f-4f7a-ab5a-2bef004164ca    | /v1/internal/ui/services?dc=dc-1                              | /dc-1/intentions |
    ------------------------------------------------------------------------------------------------------------------------------------------------------------------------
  Scenario: The node detail page calls the correct API endpoints
    When I visit the node page for yaml
    ---
      dc: dc-1
      node: node-0
      ---
    Then the last GET requests were like yaml
    ---
      - /v1/catalog/datacenters
      - /v1/internal/ui/node/node-0?dc=dc-1
      - /v1/coordinate/nodes?dc=dc-1
      - /v1/session/node/node-0?dc=dc-1
    ---
  Scenario: The kv detail page calls the correct API endpoints
    When I visit the kv page for yaml
    ---
      dc: dc-1
      kv: keyname
      ---
    Then the last GET requests were like yaml
    ---
      - /v1/catalog/datacenters
      - /v1/kv/keyname?dc=dc-1
      - /v1/session/info/ee52203d-989f-4f7a-ab5a-2bef004164ca?dc=dc-1
    ---
  Scenario: The intention detail page calls the correct API endpoints
    When I visit the intention page for yaml
    ---
      dc: dc-1
      intention: intention
      ---
    Then the last GET requests were like yaml
    ---
      - /v1/catalog/datacenters
      - /v1/connect/intentions/intention?dc=dc-1
      - /v1/internal/ui/services?dc=dc-1
    ---

  Scenario: Clicking a [Item] in the [Model] listing and canceling
    When I visit the [Model] page for yaml
    ---
      dc: dc-1
    ---
    When I click [Item] on the [Model]
    Then the url should be [URL]
    And I click "[type=reset]"
    Then the url should be [Back]
  Where:
    --------------------------------------------------------------------------------------------------------
    | Item      | Model      | URL                                                      | Back             |
    | kv        | kvs        | /dc-1/kv/necessitatibus-0/edit                           | /dc-1/kv         |
    | acl       | acls       | /dc-1/acls/anonymous                                     | /dc-1/acls       |
    | intention | intentions | /dc-1/intentions/ee52203d-989f-4f7a-ab5a-2bef004164ca    | /dc-1/intentions |
    --------------------------------------------------------------------------------------------------------
@ignore
  Scenario: Clicking items in the listings, without depending on the salt ^
    Then ok
  Scenario: Clicking create in the [Model] listing
    When I visit the [Model] page for yaml
    ---
      dc: dc-1
    ---
    When I click create
    Then the url should be [URL]
    And I click "[data-test-back]"
    Then the url should be [Back]
  Where:
    ------------------------------------------------------------------------
    | Item      | Model      | URL                      | Back             |
    | kv        | kvs        | /dc-1/kv/create          | /dc-1/kv         |
    | acl       | acls       | /dc-1/acls/create        | /dc-1/acls       |
    | intention | intentions | /dc-1/intentions/create  | /dc-1/intentions |
    ------------------------------------------------------------------------
@ignore
  Scenario: Using I click on should change the currentPage ^
    Then ok
