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
  Scenario: Default allow is set to true
    Given 1 topology model from yaml
    ---
      FilteredByACLs: false
      TransparentProxy: false
      Downstreams:
        - Name: db-1
          Namespace: default
          Datacenter: datacenter
          Intention:
            Allowed: false
      Upstreams:
        - Name: db-2
          Namespace: default
          Datacenter: datacenter
          Intention:
            Allowed: false
    ---
    And the default ACL policy is "allow"
    When I visit the service page for yaml
    ---
      dc: datacenter
      service: web
    ---
    Then the url should be /datacenter/services/web/topology
    And I see the tabs.topologyTab.defaultAllowNotice object
  Scenario: A Downstream service has a wildcard intention
    Given 1 topology model from yaml
    ---
      FilteredByACLs: true
      TransparentProxy: false
      Downstreams:
        - Name: db-1
          Namespace: default
          Datacenter: datacenter
          Intention:
            Allowed: true
            HasExact: false
      Upstreams:
        - Name: db-2
          Namespace: default
          Datacenter: datacenter
          Intention:
            Allowed: false
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




