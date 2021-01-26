@setupApplicationTest
Feature: page-navigation
  In order to view all the data in consul
  As a user
  I should be able to visit every page and view data in a HTML from the API
  Background:
    Given 1 datacenter model with the value "dc-1"
  Scenario: Visiting the index page
    When I visit the index page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services
    Then a GET request was made to "/v1/internal/ui/services?dc=dc-1&ns=@namespace"
  Scenario: Clicking [Link] in the navigation takes me to [URL]
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    When I click [Link] on the navigation
    Then the url should be [URL]
    Then a GET request was made to "[Endpoint]"
  Where:
    -------------------------------------------------------------------------------------
    | Link       | URL               | Endpoint                                         |
    | nodes      | /dc-1/nodes       | /v1/internal/ui/nodes?dc=dc-1&ns=@namespace      |
    | kvs        | /dc-1/kv          | /v1/kv/?keys&dc=dc-1&separator=%2F&ns=@namespace |
    | tokens       | /dc-1/acls/tokens | /v1/acl/tokens?dc=dc-1&ns=@namespace             |
    # | settings   | /settings         | /v1/catalog/datacenters                         |
    -------------------------------------------------------------------------------------
  Scenario: Clicking a [Item] in the [Model] listing and back again
    When I visit the [Model] page for yaml
    ---
      dc: dc-1
    ---
    When I click [Item] on the [Model]
    Then the url should be [URL]
    Then a GET request was made to "[Endpoint]"
    And I click "[data-test-back]"
    Then the url should be [Back]
  Where:
    -------------------------------------------------------------------------------------------------------------------------------------
    | Item | Model | URL                       | Endpoint                                                                    | Back     |
    | kv   | kvs   | /dc-1/kv/0-key-value/edit | /v1/session/info/ee52203d-989f-4f7a-ab5a-2bef004164ca?dc=dc-1&ns=@namespace | /dc-1/kv |
    -------------------------------------------------------------------------------------------------------------------------------------
  Scenario: The node detail page calls the correct API endpoints
    When I visit the node page for yaml
    ---
      dc: dc-1
      node: node-0
      ---
    Then the url should be /dc-1/nodes/node-0/health-checks
    Then the last GET requests included from yaml
    ---
      - /v1/catalog/datacenters
      - /v1/namespaces
      - /v1/internal/ui/node/node-0?dc=dc-1&ns=@namespace
      - /v1/coordinate/nodes?dc=dc-1
    ---
  Scenario: The kv detail page calls the correct API endpoints
    When I visit the kv page for yaml
    ---
      dc: dc-1
      kv: keyname
      ---
    Then the url should be /dc-1/kv/keyname/edit
    Then the last GET requests included from yaml
    ---
      - /v1/catalog/datacenters
      - /v1/namespaces
      - /v1/kv/keyname?dc=dc-1&ns=@namespace
      - /v1/session/info/ee52203d-989f-4f7a-ab5a-2bef004164ca?dc=dc-1&ns=@namespace
    ---
  Scenario: The policies page/tab calls the correct API endpoints
    When I visit the policies page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/policies
    Then the last GET requests included from yaml
    ---
      - /v1/catalog/datacenters
      - /v1/namespaces
      - /v1/acl/policies?dc=dc-1&ns=@namespace
    ---

  Scenario: Clicking a [Item] in the [Model] listing and cancelling
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
    | kv        | kvs        | /dc-1/kv/0-key-value/edit                           | /dc-1/kv         |
    # | acl       | acls       | /dc-1/acls/anonymous                                     | /dc-1/acls       |
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
    -----------------------------------------------------------------------------
    | Item      | Model      | URL                        | Back                |
    | kv        | kvs        | /dc-1/kv/create            | /dc-1/kv            |
    # | acl       | acls       | /dc-1/acls/create         | /dc-1/acls         |
    | intention | intentions | /dc-1/intentions/create    | /dc-1/intentions    |
    | token     | tokens     | /dc-1/acls/tokens/create   | /dc-1/acls/tokens   |
    | policy    | policies   | /dc-1/acls/policies/create | /dc-1/acls/policies |
    -----------------------------------------------------------------------------
@ignore
  Scenario: Using I click on should change the currentPage ^
    Then ok
