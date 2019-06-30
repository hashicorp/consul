@setupApplicationTest
Feature: components / copy-button
  Background:
    Given 1 datacenter model with the value "dc-1"
  Scenario: Clicking the copy button
    When I visit the node page for yaml
    ---
      dc: dc-1
      node: node-0
      ---
    Then the url should be /dc-1/nodes/node-0
    When I click ".healthcheck-output:nth-child(1) button.copy-btn"
    Then I see the text "Copied output!" in ".healthcheck-output:nth-child(1) p.feedback-dialog-out"
