@setupApplicationTest
Feature: deleting: Deleting from the listing and the detail page with confirmation
  Scenario: Deleting a [Model] from the [Model] listing page
    Given 1 datacenter model with the value "datacenter"
    And 1 [Model] model from json
    ---
      [Data]
    ---
    When I visit the [Model]s page for yaml
    ---
      dc: datacenter
    ---
    And I click actions on the [Model]s
    And I click delete on the [Model]s
    And I click confirmDelete on the [Model]s
    Then a [Method] request is made to "[URL]"
    When I visit the [Model] page for yaml
    ---
      dc: datacenter
      [Slug]
    ---
    And I click delete
    And I click confirmDelete
    Then a [Method] request is made to "[URL]"
  Where:
    ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
    | Model     | Method | URL                                                                       | Data                                                                 | Slug                                            |
    | acl       | PUT    | /v1/acl/destroy/something?dc=datacenter                                   | {"Name": "something", "ID": "something"}                             | acl: something                                  |
    | kv        | DELETE | /v1/kv/key-name?dc=datacenter                                             | ["key-name"]                                                         | kv: key-name                                    |
    | intention | DELETE | /v1/connect/intentions/ee52203d-989f-4f7a-ab5a-2bef004164ca?dc=datacenter | {"SourceName": "name", "ID": "ee52203d-989f-4f7a-ab5a-2bef004164ca"} | intention: ee52203d-989f-4f7a-ab5a-2bef004164ca |
    ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
@ignore
  Scenario: Sort out the wide tables ^
    Then ok
