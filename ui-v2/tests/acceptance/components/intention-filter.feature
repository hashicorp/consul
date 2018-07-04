@setupApplicationTest
Feature: components / intention filter: Intention Filter
  In order to find the intention I'm looking for easier
  As a user
  I should be able to filter by 'policy' (allow/deny) and freetext search tokens by source and destination
  Scenario: Filtering [Model]
    Given 1 datacenter model with the value "dc-1"
    And 2 [Model] models
    When I visit the [Page] page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be [Url]

    Then I see 2 [Model] models
    And I see allIsSelected on the filter

    When I click allow on the filter
    Then I see allowIsSelected on the filter
    And I see 1 [Model] model
    And I see 1 [Model] model with the action "allow"

    When I click deny on the filter
    Then I see denyIsSelected on the filter
    And I see 1 [Model] model
    And I see 1 [Model] model with the action "deny"

    When I click all on the filter
    Then I see 2 [Model] models
    Then I see allIsSelected on the filter
    Then I fill in with yaml
    ---
    s: alarm
    ---
    And I see 1 [Model] model
    And I see 1 [Model] model with the source "alarm"
    Then I fill in with yaml
    ---
    s: feed
    ---
    And I see 1 [Model] model
    And I see 1 [Model] model with the destination "feed"
    Then I fill in with yaml
    ---
    s: transmitter
    ---
    And I see 2 [Model] models
    And I see 1 [Model] model with the source "transmitter"
    And I see 1 [Model] model with the destination "transmitter"

  Where:
    ---------------------------------------------
    | Model     | Page       | Url              |
    | intention | intentions | /dc-1/intentions |
    ---------------------------------------------
