@setupApplicationTest
Feature: dc forwarding
  In order to arrive at a useful page when only specifying a dc in the url
  As a user
  I should be redirected to the services page for the dc
  Scenario: Arriving at the datacenter index page with no other url info
    Given 1 datacenter model with the value "datacenter"
    When I visit the dcs page for yaml
    ---
    dc: datacenter
    ---
    Then the url should be /datacenter/services
