@setupApplicationTest
@notNamespaceable
Feature: dc / intentions / sorting
  Scenario: Sorting Intentions
    Given 1 datacenter model with the value "dc-1"
    And 6 intention models from yaml
    ---
    - Action: "allow"
    - Action: "allow"
    - Action: "deny"
    - Action: "deny"
    - Action: "allow"
    - Action: "deny"
    ---
    When I visit the intentions page for yaml
    ---
      dc: dc-1
    ---
    Then I see 6 intention models on the intentionList component
    When I click action on the sort
    Then I see action on the intentionList.intentions vertically like yaml
    ---
    - "deny"
    - "deny"
    - "deny"
    - "allow"
    - "allow"
    - "allow"
    ---
    When I click action on the sort
    Then I see action on the intentionList.intentions vertically like yaml
    ---
    - "allow"
    - "allow"
    - "allow"
    - "deny"
    - "deny"
    - "deny"
    ---

