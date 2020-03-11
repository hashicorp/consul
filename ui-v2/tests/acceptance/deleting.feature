@setupApplicationTest
Feature: deleting: Deleting items with confirmations, success and error notifications
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
    --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
    | Edit     | Listing     | Method | URL                                                                             | Data                                                                 |
    | kv        | kvs        | DELETE | /v1/kv/key-name?dc=datacenter&ns=@!namespace                                    | ["key-name"]                                                         |
    | intention | intentions | DELETE | /v1/connect/intentions/ee52203d-989f-4f7a-ab5a-2bef004164ca?dc=datacenter       | {"SourceName": "name", "ID": "ee52203d-989f-4f7a-ab5a-2bef004164ca"} |
    | token     | tokens     | DELETE | /v1/acl/token/001fda31-194e-4ff1-a5ec-589abf2cafd0?dc=datacenter&ns=@!namespace | {"AccessorID": "001fda31-194e-4ff1-a5ec-589abf2cafd0"}               |
    # | acl      | acls       | PUT    | /v1/acl/destroy/something?dc=datacenter                                        | {"Name": "something", "ID": "something"}                             |
    --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
  Scenario: Deleting a [Model] from the [Model] detail page
    When I visit the [Model] page for yaml
    ---
      dc: datacenter
      [Slug]
    ---
    And I click delete
    And I click confirmDelete
    Then a [Method] request was made to "[URL]"
    And "[data-notification]" has the "notification-delete" class
    And "[data-notification]" has the "success" class
    When I visit the [Model] page for yaml
    ---
      dc: datacenter
      [Slug]
    ---
    Given the url "[URL]" responds with a 500 status
    And I click delete
    And I click confirmDelete
    And "[data-notification]" has the "notification-delete" class
    And "[data-notification]" has the "error" class
  Where:
    -----------------------------------------------------------------------------------------------------------------------------------------------------------
    | Model     | Method | URL                                                                              | Slug                                            |
    | kv        | DELETE | /v1/kv/key-name?dc=datacenter&ns=@!namespace                                     | kv: key-name                                    |
    | intention | DELETE | /v1/connect/intentions/ee52203d-989f-4f7a-ab5a-2bef004164ca?dc=datacenter        | intention: ee52203d-989f-4f7a-ab5a-2bef004164ca |
    | token     | DELETE | /v1/acl/token/001fda31-194e-4ff1-a5ec-589abf2cafd0?dc=datacenter&ns=@!namespace  | token: 001fda31-194e-4ff1-a5ec-589abf2cafd0     |
    # | acl       | PUT    | /v1/acl/destroy/something?dc=datacenter                                      | acl: something                                  |
    -----------------------------------------------------------------------------------------------------------------------------------------------------------
@ignore
  Scenario: Sort out the wide tables ^
    Then ok
