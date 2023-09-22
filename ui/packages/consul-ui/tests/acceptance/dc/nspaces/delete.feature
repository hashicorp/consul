@setupApplicationTest
@onlyNamespaceable
Feature: dc / nspaces / delete: Deleting items with confirmations, success and error notifications
  In order to delete items in consul
  As a user
  I should be able to delete items, get confirmation or a error notification that it has or has not been deleted
  Background:
    Given 1 datacenter model with the value "datacenter"
  Scenario: Deleting a [Edit] model from the [Listing] listing page
    Given 1 [Edit] model from json
    ---
      [Data]
    ---
    When I visit the [Listing] page for yaml
    ---
      dc: datacenter
    ---
    And I click actions on the [Listing]
    And I click delete on the [Listing]
    And I click confirmDelete on the [Listing]
    Then a [Method] request was made to "[URL]"
    And "[data-notification]" has the "notification-delete" class
    And "[data-notification]" has the "success" class
  Where:
    --------------------------------------------------------------------------------------------------------
    | Edit     | Listing     | Method | URL                                      | Data                    |
    | nspace    | nspaces    | DELETE | /v1/namespace/a-namespace?dc=datacenter  | {"Name": "a-namespace"} |
    --------------------------------------------------------------------------------------------------------
  Scenario: Deleting a nspace from the nspace detail page with success
    When I visit the nspace page for yaml
    ---
      dc: datacenter
      namespace: a-namespace
    ---
    And I click delete
    And I click confirmDelete
    Then a DELETE request was made to "/v1/namespace/a-namespace?dc=datacenter"
    And "[data-notification]" has the "notification-delete" class
    And "[data-notification]" has the "success" class
  Scenario: Deleting a [Model] from the [Model] detail page with error
    When I visit the [Model] page for yaml
    ---
      dc: datacenter
      [Slug]
    ---
    Given the url "[URL]" responds with a 500 status
    And I click delete
    And I click confirmDelete
    And "[data-notification]" has the "notification-update" class
    And "[data-notification]" has the "error" class
  Where:
    -------------------------------------------------------------------------------------------
    | Model     | Method | URL                                      | Slug                    |
    | nspace    | DELETE | /v1/namespace/a-namespace?dc=datacenter  | namespace: a-namespace  |
    -------------------------------------------------------------------------------------------
