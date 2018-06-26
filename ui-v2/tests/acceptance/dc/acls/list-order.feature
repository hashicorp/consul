@setupApplicationTest
Feature: dc / acls / list-order
  In order to be able to find ACL tokens easier
  As a user
  I want to see the ACL listed alphabetically by Name

  Scenario: I have 10 randomly sorted tokens
    Given 1 datacenter model with the value "datacenter"
    And 10 acl model from yaml
    ---
      - Name: zz
      - Name: 123
      - Name: aa
      - Name: 9857
      - Name: sfgr
      - Name: foo
      - Name: bar
      - Name: xft
      - Name: z-35y
      - Name: __acl
    ---
    When I visit the acls page for yaml
    ---
      dc: datacenter
    ---
    Then I see name on the acls like yaml
    ---
      - __acl
      - 123
      - 9857
      - aa
      - bar
      - foo
      - sfgr
      - xft
      - z-35y
      - zz
    ---
