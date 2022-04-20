@setupApplicationTest
Feature: dc / intentions / index
  Scenario: Viewing intentions in the listing
    Given 1 datacenter model with the value "dc-1"
    And 3 intention models
    When I visit the intentions page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/intentions
    And the title should be "Intentions - Consul"
    Then I see 3 intention models on the intentionList component
  Scenario: Viewing intentions with no write access
    Given 1 datacenter model with the value "dc-1"
    And 3 intention models
    And permissions from yaml
    ---
    intention:
      write: false
    ---
    When I visit the intentions page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/intentions
    And I see create
# We currently hardcode intention write to true until the API does what we need
# Once we can use this as we need we'll be able to un-hardcode And this test
# will fail again, at which point we can remove the above assertion and
# uncomment the below one
    # And I don't see create
  Scenario: Viewing intentions in the listing live updates
    Given 1 datacenter model with the value "dc-1"
    Given 3 intention models
    And a network latency of 100
    When I visit the intentions page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/intentions
    And pause until I see 3 intention models on the intentionList component
    And an external edit results in 5 intention models
    And pause until I see 5 intention models on the intentionList component
    And an external edit results in 1 intention model
    And pause until I see 1 intention models on the intentionList component
    And an external edit results in 0 intention models
    And pause until I see 0 intention models on the intentionList component
  Scenario: Viewing intentions in the listing with CRDs
    Given 1 datacenter model with the value "dc-1"
    And 1 intention models from yaml
    ---
    Meta:
      external-source: kubernetes
    ---
    When I visit the intentions page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/intentions
    Then I see customResourceNotice on the intentionList
  Scenario: Viewing intentions in the listing without CRDs
    Given 1 datacenter model with the value "dc-1"
    And 1 intention models from yaml
    ---
    Meta:
      external-source: consul
    ---
    When I visit the intentions page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/intentions
    Then I don't see customResourceNotice on the intentionList
