@setupApplicationTest
Feature: page-navigation
  In order to view all the data in consul
  As a user
  I should be able to visit every page and view data in a HTML from the API
  Background:
    Given 1 datacenter model with the value "dc1"
  Scenario: Visiting the index page
    When I visit the index page for yaml
    ---
      dc: dc1
    ---
    Then the url should be /dc1/services
  Scenario: Clicking [Link] in the navigation takes me to [URL]
    When I visit the services page for yaml
    ---
      dc: dc1
    ---
    When I click [Link] on the navigation
    Then the url should be [URL]
    Then a GET request was made to "[Endpoint]"
  Where:
    ---------------------------------------------------------------------------------------------------
    | Link       | URL               | Endpoint                                                       |
    | nodes      | /dc1/nodes       | /v1/internal/ui/nodes?dc=dc1&ns=@namespace      |
  # FIXME
    # | kvs        | /dc1/kv          | /v1/kv/?keys&dc=dc1&separator=%2F&ns=@namespace               |
    | tokens       | /dc1/acls/tokens | /v1/acl/tokens?dc=dc1&ns=@namespace                           |
    # | settings   | /settings         | /v1/catalog/datacenters                                      |
    ---------------------------------------------------------------------------------------------------
  # FIXME
  @ignore
  Scenario: Clicking a [Item] in the [Model] listing and back again
    When I visit the [Model] page for yaml
    ---
      dc: dc1
    ---
    When I click [Item] on the [Model]
    Then the url should be [URL]
    Then a GET request was made to "[Endpoint]"
    And I click "[data-test-back]"
    Then the url should be [Back]
  Where:
    -------------------------------------------------------------------------------------------------------------------------------------
    | Item | Model | URL                       | Endpoint                                                                    | Back     |
    | kv   | kvs   | /dc1/kv/0-key-value/edit | /v1/session/info/ee52203d-989f-4f7a-ab5a-2bef004164ca?dc=dc1&ns=@namespace | /dc1/kv |
    -------------------------------------------------------------------------------------------------------------------------------------
  Scenario: The node detail page calls the correct API endpoints
    When I visit the node page for yaml
    ---
      dc: dc1
      node: node-0
      ---
    Then the url should be /dc1/nodes/node-0/health-checks
    Then the last GET requests included from yaml
    ---
      - /v1/catalog/datacenters
      - /v1/internal/ui/node/node-0?dc=dc1&ns=@namespace
      - /v1/coordinate/nodes?dc=dc1
    ---
  # FIXME
  @ignore
  Scenario: The kv detail page calls the correct API endpoints
    When I visit the kv page for yaml
    ---
      dc: dc1
      kv: keyname
      ---
    Then the url should be /dc1/kv/keyname/edit
    Then the last GET requests included from yaml
    ---
      - /v1/catalog/datacenters
      - /v1/kv/keyname?dc=dc1&ns=@namespace
      - /v1/session/info/ee52203d-989f-4f7a-ab5a-2bef004164ca?dc=dc1&ns=@namespace
    ---
  Scenario: The policies page/tab calls the correct API endpoints
    When I visit the policies page for yaml
    ---
      dc: dc1
    ---
    Then the url should be /dc1/acls/policies
    Then the last GET requests included from yaml
    ---
      - /v1/catalog/datacenters
      - /v1/acl/policies?dc=dc1&ns=@namespace
    ---

  # FIXME
  @ignore
  Scenario: Clicking a [Item] in the [Model] listing and cancelling
    When I visit the [Model] page for yaml
    ---
      dc: dc1
    ---
    When I click [Item] on the [Model]
    Then the url should be [URL]
    And I click "[type=reset]"
    Then the url should be [Back]
  Where:
    -----------------------------------------------------------------------------------------------
    | Item      | Model      | URL                                                      | Back    |
    | kv        | kvs        | /dc1/kv/0-key-value/edit                                 | /dc1/kv |
    -----------------------------------------------------------------------------------------------
@ignore
  Scenario: Clicking items in the listings, without depending on the salt ^
    Then ok
  Scenario: Clicking create in the [Model] listing
    When I visit the [Model] page for yaml
    ---
      dc: dc1
    ---
    When I click create
    Then the url should be [URL]
    And I click "[data-test-back]"
    Then the url should be [Back]
  Where:
    ---------------------------------------------------------------------------
    | Item      | Model      | URL                       | Back               |
  # FIXME
    # | kv        | kvs        | /dc1/kv/create            | /dc1/kv            |
    | intention | intentions | /dc1/intentions/create    | /dc1/intentions    |
    | token     | tokens     | /dc1/acls/tokens/create   | /dc1/acls/tokens   |
    | policy    | policies   | /dc1/acls/policies/create | /dc1/acls/policies |
    ---------------------------------------------------------------------------
@ignore
  Scenario: Using I click on should change the currentPage ^
    Then ok
