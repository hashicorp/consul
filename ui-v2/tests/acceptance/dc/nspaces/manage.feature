@setupApplicationTest
@onlyNamespaceable
Feature: dc / nspaces / manage : Managing Namespaces
  Scenario:
    Given settings from yaml
    ---
    consul:token:
      SecretID: secret
      AccessorID: accessor
      Namespace: default
    ---
    And 1 datacenter models from yaml
    ---
      - dc-1
    ---
    And 6 service models
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services
    Then I see 6 service models
    # In order to test this properly you have to click around a few times
    # between services and nspace management
    When I click nspace on the navigation
    And I click manageNspaces on the navigation
    Then the url should be /dc-1/namespaces
    And I don't see manageNspacesIsVisible on the navigation
    When I click services on the navigation
    Then the url should be /dc-1/services
    When I click nspace on the navigation
    And I click manageNspaces on the navigation
    Then the url should be /dc-1/namespaces
    And I don't see manageNspacesIsVisible on the navigation
