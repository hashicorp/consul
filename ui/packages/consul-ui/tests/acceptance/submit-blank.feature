@setupApplicationTest
Feature: submit-blank
  In order to prevent form's being saved without values
  As a user
  I shouldn't be able to submit a blank form
  Scenario: Visiting a blank form for [Model]
    Given 1 datacenter model with the value "datacenter"
    When I visit the [Model] page for yaml
    ---
      dc: datacenter
    ---
    Then the url should be /datacenter/[Slug]/create
    And I submit
    Then the url should be /datacenter/[Slug]/create
  Where:
    --------------------------
    | Model     | Slug       |
    | kv        | kv         |
    --------------------------
@ignore
  Scenario: The button is disabled
    Then ok

