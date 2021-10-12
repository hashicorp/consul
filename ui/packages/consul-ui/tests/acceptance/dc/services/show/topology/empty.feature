@setupApplicationTest
Feature: dc / services / show / topology / empty
  Background:
    Given 1 datacenter model with the value "datacenter"
    And the local datacenter is "datacenter"
    And 1 node model
    And 1 service model from yaml
    ---
    - Service:
        Name: web
        Kind: ~
    ---
    And 1 topology model from yaml
    ---
      FilteredByACLs: false
      TransparentProxy: false
      DefaultAllow: false
      WildcardIntention: false
      Downstreams: []
      Upstreams: []
    ---
  Scenario: No Dependencies
    When I visit the service page for yaml
    ---
      dc: datacenter
      service: web
    ---
    Then the url should be /datacenter/services/web/topology
    And I see the text "No downstreams." in "#downstream-container > a > p"
    And I see the text "No upstreams." in "#upstream-container > a > p"
    And I see the tabs.topologyTab.noDependencies object
  # Scenario: ACLs disabled
  #   Given ACLs are disabled
  #   When I visit the service page for yaml
  #   ---
  #     dc: datacenter
  #     service: web
  #   ---
  #   Then the url should be /datacenter/services/web/topology
  #   And I see the text "Downstreams unknown." in "#downstream-container > a > p"
  #   And I see the text "Upstreams unknown." in "#upstream-container > a > p"
  #   And I see the tabs.topologyTab.aclsDisabled object
