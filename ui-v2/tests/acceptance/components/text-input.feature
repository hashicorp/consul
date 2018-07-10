@setupApplicationTest
Feature: components / text-input: Text input
  Background:
    Given 1 datacenter model with the value "dc-1"
  Scenario:
    When I visit the [Page] page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be [Url]
    Then I fill in with json
    ---
    [Data]
    ---
    Then I see submitIsEnabled
  Where:
    --------------------------------------------------------------------------------
    | Page       | Url                    | Data                                   |
    | kv        | /dc-1/kv/create         | {"additional": "hi", "value": "there"} |
    | acl       | /dc-1/acls/create       | {"name": "hi"}                         |
    --------------------------------------------------------------------------------
