@setupApplicationTest
Feature: KV Update
  Scenario: Update to [Name] change value to [Value]
    Given 1 datacenter model with the value "datacenter"
    And 1 kv model from yaml
    ---
      Key: [Name]
    ---
    When I visit the kv page for yaml
    ---
      dc: datacenter
      kv: [Name]
    ---
    Then I type with yaml
    ---
      value: [Value]
    ---
    And I submit
    Then a PUT request is made to "/v1/kv/[Name]?dc=datacenter" with the body "[Value]"
  Where:
      ------------------------------------
      | Name              | Value        |
      # | key               | value        |
      # | key-name          | a value      |
      | folder/key-name   | a value      |
      ------------------------------------
# @ignore
  # Scenario: Rules can be edited/updated
    # Then ok
# @ignore
  # Scenario: The feedback dialog says success or failure
    # Then ok
