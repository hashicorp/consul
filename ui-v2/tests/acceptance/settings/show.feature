@setupApplicationTest
@notNamespaceable

Feature: settings / show: Show Settings Page
  Scenario:
    Given 1 datacenter model with the value "datacenter"
    When I visit the settings page
    Then the url should be /setting
    And the title should be "Settings - Consul"
