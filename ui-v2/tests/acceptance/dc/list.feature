@setupApplicationTest
Feature: List Models
  Scenario: Listing [Model]
    Given 1 datacenter model with the value "dc-1"
    And 3 [Model] models
    When I visit the [Page] page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be [Url]

    Then I see 3 [Model] models
  Where:
    -------------------------------------------------
    | Model   | Page     | Url                      |
    | service | services | /dc-1/services           |
    | node    | nodes    | /dc-1/nodes              |
    | kv      | kvs      | /dc-1/kv                 |
    | acl     | acls     | /dc-1/acls               |
    -------------------------------------------------
