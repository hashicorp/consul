@setupApplicationTest
Feature: dc / intentions / permissions / create: Intention Permission Create
  Scenario:
    Given 1 datacenter model with the value "datacenter"
    When I visit the intention page for yaml
    ---
      dc: datacenter
    ---
    Then the url should be /datacenter/intentions/create
    And the title should be "New Intention - Consul"
    # Specifically set L7
    And I click ".value-"

    And I click the permissions.create object
    And I click the permissions.form.Action.option.Deny object
    And I click the permissions.form.PathType object
    And I click the permissions.form.PathType.option.PrefixedBy object
    And I fillIn the permissions.form.Path object with value "/path"
    And I fillIn the permissions.form.Headers.form.Name object with value "Name"
    And I fillIn the permissions.form.Headers.form.Value object with value "/path/name"
    And I click the permissions.form.Headers.form.submit object
    And I see 1 of the permissions.form.Headers.list.intentionPermissionHeaders objects
    And I click the permissions.form.submit object
    And I see 1 of the permissions.list.intentionPermissions objects
