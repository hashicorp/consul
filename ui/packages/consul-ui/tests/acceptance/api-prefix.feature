@setupApplicationTest
Feature: api-prefix
  Scenario:
    Given 1 datacenter model with the value "dc1"
    And an API prefix of "/prefixed-api"
    When I visit the index page
    Then a GET request was made to "/prefixed-api/v1/catalog/datacenters"
