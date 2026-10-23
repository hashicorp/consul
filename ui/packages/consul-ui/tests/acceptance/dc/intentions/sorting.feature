@setupApplicationTest
@notNamespaceable
Feature: dc / intentions / sorting
  Scenario: Sorting Intentions
    Given 1 datacenter model with the value "dc-1"
    And 3 intention models from yaml
    ---
    - SourceName: "service-B"
    - SourceName: "service-D"
    - SourceName: "service-A"
    ---
    When I visit the intentions page for yaml
    ---
      dc: dc-1
    ---
    Then I see 3 intention models on the intentionList component
    # ascending (A-Z)
    When I click source on the sort
    Then I see source on the intentionList.intentions vertically like yaml
    ---
    - "service-A"
    - "service-B"
    - "service-D"
    ---
    # descending (Z-A)
    When I click source on the sort
    Then I see source on the intentionList.intentions vertically like yaml
    ---
    - "service-D"
    - "service-B"
    - "service-A"
    ---

