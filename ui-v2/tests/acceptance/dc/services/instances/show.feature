@setupApplicationTest
Feature: dc / services / instances / show: Show Service Instance
  Scenario: A Service instance has no Proxy
    Given 1 datacenter model with the value "dc1"
    And 1 service model from yaml
    ---
    - Service:
        ID: service-0-with-id
        Tags: ['Tag1', 'Tag2']
        Meta:
          external-source: nomad
      Checks:
        - Name: Service check
          ServiceID: service-0
          Output: Output of check
          Status: passing
        - Name: Service check
          ServiceID: service-0
          Output: Output of check
          Status: warning
        - Name: Service check
          ServiceID: service-0
          Output: Output of check
          Status: critical
        - Name: Node check
          ServiceID: ""
          Output: Output of check
          Status: passing
        - Name: Node check
          ServiceID: ""
          Output: Output of check
          Status: warning
        - Name: Node check
          ServiceID: ""
          Output: Output of check
          Status: critical
    ---
    And 1 proxy model from yaml
    ---
    - ServiceProxy:
        DestinationServiceName: service-1
        DestinationServiceID: ~
    ---
    When I visit the instance page for yaml
    ---
      dc: dc1
      service: service-0
      id: service-0-with-id
    ---
    Then the url should be /dc1/services/service-0/service-0-with-id
    Then I don't see type on the proxy

    Then I see externalSource like "nomad"

    And I don't see upstreams on the tabs
    And I see serviceChecksIsSelected on the tabs
    And I see 3 of the serviceChecks object

    When I click nodeChecks on the tabs
    And I see nodeChecksIsSelected on the tabs
    And I see 3 of the nodeChecks object

    When I click tags on the tabs
    And I see tagsIsSelected on the tabs

    Then I see the text "Tag1" in "[data-test-tags] span:nth-child(1)"
    Then I see the text "Tag2" in "[data-test-tags] span:nth-child(2)"

