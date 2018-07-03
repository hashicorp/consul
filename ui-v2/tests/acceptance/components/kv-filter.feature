@setupApplicationTest
Feature: components / kv-filter
  Scenario: Filtering using the freetext filter
    Given 1 datacenter model with the value "dc-1"
    And 2 [Model] models from yaml
    ---
      - hi
      - there
    ---
    When I visit the [Page] page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be [Url]
    Then I fill in with yaml
    ---
    s: [Text]
    ---
    And I see 1 [Model] model with the name "[Text]"

  Where:
    ----------------------------------------------------------------
    | Model   | Page     | Url        | Text            | Property |
    | kv      | kvs      | /dc-1/kv   | hi              | name     |
    | kv      | kvs      | /dc-1/kv   | there           | name     |
    ----------------------------------------------------------------
