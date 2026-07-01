@setupApplicationTest
Feature: dc / acls / auth-methods / index: ACL Auth Methods List

  Scenario:
    Given 1 datacenter model with the value "dc-1"
    And 3 authMethod models
    When I visit the authMethods page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/auth-methods
    Then I see 3 authMethod models
    And the title should be "Auth Methods - Consul"
  Scenario: Searching the Auth Methods
    Given 1 datacenter model with the value "dc-1"
    And 3 authMethod models from yaml
    ---
    - Name: kube
      DisplayName: minikube
    - Name: agent
      DisplayName: ''
    - Name: node
      DisplayName: mininode
    ---
    When I visit the authMethods page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/acls/auth-methods
    Then I see 3 authMethod models
    Then I type "kube" into "input[type=search]"
    And I see 1 authMethod model
    And I see 1 authMethod model with the name "minikube"
    Then I type "agent" into "input[type=search]"
    And I see 1 authMethod model
    And I see 1 authMethod model with the name "agent"
    Then I type "ode" into "input[type=search]"
    And I see 1 authMethod model
    And I see 1 authMethod model with the name "mininode"
