@setupApplicationTest
Feature: dc / kvs / list-order
  In order to be able to find key values easier
  As a user
  I want to see the Key/Values listed alphabetically

  Scenario: I have 19 folders
    Given 1 datacenter model with the value "datacenter"
    And 19 kv models from yaml
    ---
      - __secretzzz/
      - a-thing-service/
      - a-thing-y-again-service/
      - a-thing-y-againzz-service/
      - a-z-search-service/
      - blood-pressure-service/
      - callToAction-items/
      - configuration/
      - content-service/
      - currentRepository-jobs/
      - currentRepository-service/
      - first-service/
      - logs-service/
      - rabmq-svc/
      - rabmqUtilities/
      - schedule-service/
      - vanApp-service/
      - vanCat-service/
      - vanTaxi-service/
    ---
    When I visit the kvs page for yaml
    ---
      dc: datacenter
    ---
    Then I see name on the kvs like yaml
    ---
      - __secretzzz/
      - a-thing-service/
      - a-thing-y-again-service/
      - a-thing-y-againzz-service/
      - a-z-search-service/
      - blood-pressure-service/
      - callToAction-items/
      - configuration/
      - content-service/
      - currentRepository-jobs/
      - currentRepository-service/
      - first-service/
      - logs-service/
      - rabmq-svc/
      - rabmqUtilities/
      - schedule-service/
      - vanApp-service/
      - vanCat-service/
      - vanTaxi-service/
    ---
