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
  Scenario: Clicking [Link] in the navigation takes me to [URL]
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    When I click [Link] on the navigation
    Then the url should be [URL]
  Where:
    ----------------------------------------
    | Link       | URL                     |
    | nodes      | /dc-1/nodes             |
    | kvs        | /dc-1/kv                |
    | acls       | /dc-1/acls              |
    | intentions | /dc-1/intentions        |
    | settings   | /settings               |
    ----------------------------------------
  Scenario: Clicking a [Item] in the [Model] listing and back again
    When I visit the [Model] page for yaml
    ---
      dc: dc-1
    ---
    When I click [Item] on the [Model]
    Then the url should be [URL]
    And I click "[data-test-back]"
    Then the url should be [Back]
  Where:
    --------------------------------------------------------------------------------------------------------
    | Item      | Model      | URL                                                      | Back             |
    | service   | services   | /dc-1/services/service-0                                 | /dc-1/services   |
    | node      | nodes      | /dc-1/nodes/node-0                                       | /dc-1/nodes      |
    | kv        | kvs        | /dc-1/kv/necessitatibus-0/edit                           | /dc-1/kv         |
    | acl       | acls       | /dc-1/acls/anonymous                                     | /dc-1/acls       |
    | intention | intentions | /dc-1/intentions/ee52203d-989f-4f7a-ab5a-2bef004164ca    | /dc-1/intentions |
    --------------------------------------------------------------------------------------------------------
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
  Scenario: Using I click on should change the currentPage ^
    Then ok
