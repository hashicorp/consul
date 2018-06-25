@setupApplicationTest
Feature: startup
  In order to give users an indication as early as possible that they are at the right place
  As a user
  I should be able to see a startup logo
  Scenario: When loading the index.html file into a browser
    Given 1 datacenter model with the value "dc-1"
    Then the url should be ''
    Then "html" has the "ember-loading" class
    When I visit the services page for yaml
    ---
      dc: dc-1
    ---
    Then the url should be /dc-1/services
    Then "html" doesn't have the "ember-loading" class


