@onlyNamespaceable
@setupApplicationTest
Feature: dc / acls / policies / as many / nspaces: As many for nspaces
  Scenario:
    Given 1 datacenter model with the value "dc1"
    And 1 nspace model from yaml
    ---
      Name: key
      ACLs:
        PolicyDefaults: ~
        RoleDefaults: ~
    ---
    When I visit the nspace page for yaml
    ---
      dc: dc1
      namespace: key
    ---
    Then the url should be /dc1/namespaces/key
    And I click policies.create
    And I don't see the "#policies [data-test-radiobutton=template_service-identity]" element
