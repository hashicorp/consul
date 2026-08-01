@setupApplicationTest
Feature: dc / kvs / create
  Scenario: Creating a root KV
    Given 1 datacenter model with the value "datacenter"
    When I visit the kv page for yaml
    ---
      dc: datacenter
    ---
    Then the url should be /datacenter/kv/create
    And the title should be "New Key / Value - Consul"
    And pause for 200
    Then I fill in with yaml
    ---
      additional: key-value
      value: value
    ---
    And I submit
    Then the url should be /datacenter/kv
    Then a PUT request was made to "/v1/kv/key-value?dc=datacenter&ns=@namespace"
    And "[data-notification]" has the "hds-toast" class
    And "[data-notification]" has the "hds-alert--color-success" class
  Scenario: Creating a folder
    Given 1 datacenter model with the value "datacenter"
    When I visit the kv page for yaml
    ---
      dc: datacenter
    ---
    Then the url should be /datacenter/kv/create
    And the title should be "New Key / Value - Consul"
    Then I fill in with yaml
    ---
      additional: key-value/
    ---
    And I submit
    Then the url should be /datacenter/kv
    Then a PUT request was made to "/v1/kv/key-value/?dc=datacenter&ns=@namespace"
    And "[data-notification]" has the "hds-toast" class
    And "[data-notification]" has the "hds-alert--color-success" class
  Scenario: Clicking create from within a folder
    Given 1 datacenter model with the value "datacenter"
    And 1 kv model from yaml
    ---
    - key-value/
    ---
    When I visit the kvs page for yaml
    ---
      dc: datacenter
    ---
    # folders expand in place, so Create inside one comes from its row menu
    And I click actions on the kvs
    And pause for 200
    And I click createInFolder on the kvs
    And I see the text "New Key / Value" in "h1"
    And I see the "[data-test-kv-key]" element
    Then I fill in with yaml
    ---
      additional: sub-key
      value: value
    ---
    # the current page object is still the list, so submit via the DOM
    And I click "main [type=submit]"
    Then a PUT request was made to "/v1/kv/key-value/sub-key?dc=datacenter&ns=@namespace"
  Scenario: Clicking create from within a just created folder
    Given 1 datacenter model with the value "datacenter"
    When I visit the kv page for yaml
    ---
      dc: datacenter
    ---
    Then the url should be /datacenter/kv/create
    And the title should be "New Key / Value - Consul"
    Then I fill in with yaml
    ---
      additional: key-value/
    ---
    Given 1 kv model from yaml
    ---
    - key-value/
    ---
    And I submit
    Then the url should be /datacenter/kv
    And I click "[data-test-actions-menu]"
    And pause for 200
    And I click "[data-test-create-in-folder]"
    And I see the text "New Key / Value" in "h1"
    And I see the text "Key/Values / key-value" in "[data-test-breadcrumbs] li"
    And I see the "[data-test-kv-key]" element
