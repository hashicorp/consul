@setupApplicationTest
@notNamespaceable

Feature: settings / show: Show Settings Page
  Scenario: I see the Blocking queries
    Given 1 datacenter model with the value "dc1"
    When I visit the settings page
    Then the url should be /settings
    # FIXME
    # And the title should be "Settings - Consul"
    And I see blockingQueries
  Scenario: Setting CONSUL_UI_DISABLE_REALTIME hides Blocking Queries
    Given 1 datacenter model with the value "datacenter"
    And settings from yaml
    ---
      CONSUL_UI_DISABLE_REALTIME: 1
    ---
    Then I have settings like yaml
    ---
      CONSUL_UI_DISABLE_REALTIME: "1"
    ---
    When I visit the settings page
    Then the url should be /settings
    # FIXME
    # And the title should be "Settings - Consul"
    And I don't see blockingQueries
