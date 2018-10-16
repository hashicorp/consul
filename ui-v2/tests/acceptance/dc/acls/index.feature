@setupApplicationTest
Feature: acl forwarding
  In order to arrive at a useful page when only specifying 'acls' in the url
  As a user
  I should be redirected to the tokens page
  Scenario: Arriving at the acl index page with no other url info
    Given 1 datacenter model with the value "datacenter"
    When I visit the acls page for yaml
    ---
    dc: datacenter
    ---
    Then the url should be /datacenter/acls/tokens
