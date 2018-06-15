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
    Then I type with yaml
    ---
    s: [Text]
    ---
    And I see 1 [Model] model with the name "[Text]"
    Then the url should be [Url]?filter=[Text]

  Where:
    ----------------------------------------------------------------
    | Model   | Page     | Url        | Text            | Property |
    | kv      | kvs      | /dc-1/kv   | hi              | name     |
    | kv      | kvs      | /dc-1/kv   | there           | name     |
    ----------------------------------------------------------------
  Scenario: Filtering using the freetext filter and a regexp
    Given 1 datacenter model with the value "dc-1"
    And 5 kv models from yaml
    ---
      - hi
      - there
      - how
      - are
      - you
    ---
    When I visit the kvs page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/kv
    Then I type with yaml
    ---
    s: hi|there
    ---
    And I see 2 kv models with the name "[Text]"
    Then the url should be /dc-1/kv?filter=hi%7Cthere

