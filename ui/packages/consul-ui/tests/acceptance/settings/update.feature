@setupApplicationTest
@ignore
Feature: settings / update: Update Settings
  In order to authenticate with an ACL token
  As a user
  I need to be able to add my token via the UI
  Scenario: I click Save without actually typing anything
    Given 1 datacenter model with the value "datacenter"
    When I visit the settings page
    Then the url should be /settings
    Then I have settings like yaml
    ---
    consul:token: ~
    ---
    And I submit
    Then I have settings like yaml
    ---
    consul:token: ''
    ---
    And the url should be /settings
    And "[data-notification]" has the "hds-toast" class
    And "[data-notification]" has the "hds-alert--color-success" class

