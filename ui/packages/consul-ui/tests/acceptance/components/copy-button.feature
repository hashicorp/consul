@setupApplicationTest
@ignore
Feature: components / copy-button
  Background:
    Given 1 datacenter model with the value "dc-1"
  Scenario: Clicking the copy button
    Given 1 node model from yaml
    ---
    ID: node-0
    Checks:
      - Name: gprc-check
        Node: node-0
        CheckID: grpc-check
        Status: passing
        Type: grpc
        Output: The output
        Notes: The notes
    ---
    When I visit the node page for yaml
    ---
      dc: dc-1
      node: node-0
    ---
    Then the url should be /dc-1/nodes/node-0/health-checks
    When I click ".healthcheck-output:nth-child(1) .copy-button button"
    Then I copied "The output"
