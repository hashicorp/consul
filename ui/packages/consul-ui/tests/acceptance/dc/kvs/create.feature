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
    Then I fill in with yaml
    ---
      additional: key-value
      value: value
    ---
    And I submit
    Then the url should be /datacenter/kv
    Then a PUT request was made to "/v1/kv/key-value?dc=datacenter&ns=@namespace"
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
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
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "success" class
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
    And I click kv on the kvs
    And I click create
    And I see the text "New Key / Value" in "h1"
    And I see the text "key-value" in "[data-test-breadcrumbs] li:nth-child(2) a"
    And I see the "[data-test-kv-key]" element
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
    And I click "[data-test-kv]"
    And I click "[data-test-create]"
    And I see the text "New Key / Value" in "h1"
    And I see the text "key-value" in "[data-test-breadcrumbs] li:nth-child(2) a"
    And I see the "[data-test-kv-key]" element
