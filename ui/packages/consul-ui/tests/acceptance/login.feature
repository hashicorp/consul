@setupApplicationTest
Feature: login
  Scenario: Logging into the login page from ACLs tokens
    Given 1 datacenter model with the value "dc-1"
    And the url "/v1/acl/tokens" responds with a 403 status
    When I visit the tokens page for yaml
    ---
    dc: dc-1
    ---
    Then the url should be /dc-1/acls/tokens
    And I click login on the navigation
    And I fill in the auth form with yaml
    ---
    SecretID: something
    ---
    And I click submit on the authdialog.form
    Then a GET request was made to "/v1/acl/token/self?dc=dc-1" from yaml
    ---
    headers:
      X-Consul-Token: something
    ---
  @onlyNamespaceable
  Scenario: Logging in via SSO
    Given 1 datacenter model with the value "dc-1"
    And SSO is enabled
    And partitions are enabled
    And 1 oidcProvider model from yaml
    ---
    - DisplayName: Okta
      Name: okta
      Kind: okta
    ---
    When I visit the services page for yaml
    ---
    dc: dc-1
    ---
    And the "okta" oidcProvider responds with from yaml
    ---
    state: state-123456789/abcdefghijklmnopqrstuvwxyz
    code: code-abcdefghijklmnopqrstuvwxyz/123456789
    ---
    And I click login on the navigation
    And I click "[data-test-tab=tab_sso] button"
    Then the "[name='partition']" input should have the value "default"
    And I type "partition" into "[name=partition]"
    And I click ".oidc-select button"
    Then a GET request was made to "/v1/internal/ui/oidc-auth-methods?dc=dc-1&ns=@namespace&partition=partition"
    And I click ".okta-oidc-provider"
    Then a POST request was made to "/v1/acl/oidc/auth-url?dc=dc-1&ns=@!namespace&partition=partition"
    And a POST request was made to "/v1/acl/oidc/callback?dc=dc-1&ns=@!namespace&partition=partition"
    And "[data-notification]" has the "notification-authorize" class
    And "[data-notification]" has the "success" class
