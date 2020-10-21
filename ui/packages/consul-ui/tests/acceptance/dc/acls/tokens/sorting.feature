@setupApplicationTest
Feature: dc / acls / tokens / sorting
  Scenario: Sorting Tokens
    Given 1 datacenter model with the value "dc-1"
    And 4 token models from yaml
    ---
    - AccessorID: "00000000-0000-0000-0000-000000000001"
      CreateTime: "2018-09-15T11:58:09.197Z"
    - AccessorID: "00000000-0000-0000-0000-000000000002"
      CreateTime: "2020-09-15T11:58:09.197Z"
    - AccessorID: "00000000-0000-0000-0000-000000000003"
      CreateTime: "2007-09-15T11:58:09.197Z"
    - AccessorID: "00000000-0000-0000-0000-000000000004"
      CreateTime: "2011-09-15T11:58:09.197Z"
    ---
    When I visit the tokens page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/tokens
    Then I see 4 token models
    When I click selected on the sort
    When I click options.1.button on the sort
    Then I see id on the tokens vertically like yaml
    ---
    - "00000000-0000-0000-0000-000000000003"
    - "00000000-0000-0000-0000-000000000004"
    - "00000000-0000-0000-0000-000000000001"
    - "00000000-0000-0000-0000-000000000002"
    ---
    When I click selected on the sort
    When I click options.0.button on the sort
    Then I see id on the tokens vertically like yaml
    ---
    - "00000000-0000-0000-0000-000000000002"
    - "00000000-0000-0000-0000-000000000001"
    - "00000000-0000-0000-0000-000000000004"
    - "00000000-0000-0000-0000-000000000003"
    ---