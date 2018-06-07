@setupApplicationTest
Feature: dc / kvs / list-order
  In order to be able to find key values easier
  As a user
  I want to see the Key/Values listed alphabetically

  Scenario: I have 19 folders
    Given 1 datacenter model with the value "datacenter"
    And 19 kv models from yaml
    ---
      - __secrets/
      - auctionCatalogue-svc/
      - auctionIntegrations-svc/
      - auctionNotifications-svc/
      - auctionSearch-svc/
      - bp-svc/
      - configurator/
      - contentStore-svc/
      - ctaRepository-import/
      - ctaRepository-job/
      - ctaRepository-svc/
      - fias-svc/
      - logs-svc/
      - rmq-svc/
      - rmqUtils/
      - schedule-svc/
      - vehicleAppraisal-svc/
      - vehicleCatalogue-svc/
      - vehicleTaxonomy-svc/
    ---
    When I visit the kvs page for yaml
    ---
      dc: datacenter
    ---
    Then I see name on the kvs like yaml
    ---
      - __secrets/
      - auctionCatalogue-svc/
      - auctionIntegrations-svc/
      - auctionNotifications-svc/
      - auctionSearch-svc/
      - bp-svc/
      - configurator/
      - contentStore-svc/
      - ctaRepository-import/
      - ctaRepository-job/
      - ctaRepository-svc/
      - fias-svc/
      - logs-svc/
      - rmq-svc/
      - rmqUtils/
      - schedule-svc/
      - vehicleAppraisal-svc/
      - vehicleCatalogue-svc/
      - vehicleTaxonomy-svc/
    ---
