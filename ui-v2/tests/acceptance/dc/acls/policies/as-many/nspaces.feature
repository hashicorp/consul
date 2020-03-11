@onlyNamespaceable
@setupApplicationTest
Feature: dc / acls / policies / as many / nspaces: As many for nspaces
  Scenario:
    Given 1 datacenter model with the value "datacenter"
    And 1 nspace model from yaml
    ---
      Name: key
      ACLs:
        PolicyDefaults: ~
        RoleDefaults: ~
    ---
    When I visit the nspace page for yaml
    ---
      dc: datacenter
      namespace: key
    ---
    Then the url should be /datacenter/namespaces/key
    And I click policies.create
    And I don't see the "#policies [data-test-radiobutton=template_service-identity]" element
