@setupApplicationTest
Feature: dc / services / show / topology / tproxy
  Background:
    Given 1 datacenter model with the value "datacenter"
    And the local datacenter is "datacenter"
    And 1 intention model from yaml
    ---
      SourceNS: default
      SourceName: web
      DestinationNS: default
      DestinationName: db
      ID: intention-id
    ---
    And 1 node model
    And 1 service model from yaml
    ---
    - Service:
        Name: web
        Kind: ~
    ---
  Scenario: Deafult allow is set to true
    Given 1 topology model from yaml
    ---
      FilteredByACLs: false
      TransparentProxy: false
      DefaultAllow: true
      WildcardIntention: false
    ---
    When I visit the service page for yaml
    ---
      dc: datacenter
      service: web
    ---
    Then the url should be /datacenter/services/web/topology
    And I see the tabs.topologyTab.defaultAllowNotice object
  Scenario: WildcardIntetions and FilteredByACLs are set to true
    Given 1 topology model from yaml
    ---
      FilteredByACLs: true
      TransparentProxy: false
      DefaultAllow: false
      WildcardIntention: true
    ---
    When I visit the service page for yaml
    ---
      dc: datacenter
      service: web
    ---
    Then the url should be /datacenter/services/web/topology
    And I see the tabs.topologyTab.filteredByACLs object
    And I see the tabs.topologyTab.wildcardIntention object
  Scenario: TProxy for a downstream is set to false
    Given 1 topology model from yaml
    ---
      FilteredByACLs: false
      TransparentProxy: false
      DefaultAllow: false
      WildcardIntention: false
      Downstreams:
        - Name: db
          Namespace: default
          Datacenter: datacenter
          Intention:
            Allowed: true
          Source: specific-intention
          TransparentProxy: false
    ---
    When I visit the service page for yaml
    ---
      dc: datacenter
      service: web
    ---
    Then the url should be /datacenter/services/web/topology
    And I see the tabs.topologyTab.notDefinedIntention object
  Scenario: TProxy for a downstream is set to true
    Given 1 topology model from yaml
    ---
      FilteredByACLs: false
      TransparentProxy: false
      DefaultAllow: false
      WildcardIntention: false
      Downstreams:
        - Name: db
          Namespace: default
          Datacenter: datacenter
          Intention:
            Allowed: true
          Source: specific-intention
          TransparentProxy: true
    ---
    When I visit the service page for yaml
    ---
      dc: datacenter
      service: web
    ---
    Then the url should be /datacenter/services/web/topology
    And I don't see the tabs.topologyTab.notDefinedIntention object




