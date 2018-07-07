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
  Scenario: Clicking [Link] in the navigation takes me to [Url]
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    When I click [Link] on the navigation
    Then the url should be [Url]
  Where:
    --------------------------------------
    | Link     | Url                     |
    | nodes    | /dc-1/nodes             |
    | kvs      | /dc-1/kv                |
    | acls     | /dc-1/acls              |
    | settings | /settings               |
    --------------------------------------
  Scenario: Clicking a [Item] in the [Model] listing
    When I visit the [Model] page for yaml
    ---
      dc: dc-1
    ---
    When I click [Item] on the [Model]
    Then the url should be [Url]
  Where:
    --------------------------------------------------------
    | Item     | Model    | Url                            |
    | service  | services | /dc-1/services/service-0       |
    | node     | nodes    | /dc-1/nodes/node-0             |
    | kv       | kvs      | /dc-1/kv/necessitatibus-0/edit |
    | acl      | acls     | /dc-1/acls/anonymous           |
    --------------------------------------------------------
@ignore
  Scenario: Clicking a kv in the kvs listing, without depending on the salt ^
    Then ok
