# WIP: Configuring Auth0 as an OIDC Auth Method in Consul Enterprise

**Until this is fully fleshed out, please read [the vault
guide](https://learn.hashicorp.com/vault/identity-access-management/oidc-auth)
first.**

## Prerequisites

To perform the tasks described in this guide, you need to have **Consul
Enterprise 1.8** or later. ACLs will need to be enabled. A sample config file
suitable for this setup in the guide can be:

```
acl {
  enabled        = true
  default_policy = "deny"

  tokens {
    master = "root"
    agent  = "root"
  }
}
```

It is assumed that the bootstrap token is passed on all commands below. In the
sample configs below that token will be `root`.

### Auth0 Account

To demonstrate an end-to-end workflow, this guide uses
[Auth0](https://auth0.com/), so create an account if you don't have one.

## Step 1: Get Auth0 credentials

If you do not have an account with Auth0, [sign
up](https://auth0.com/signup?&signUpData=%7B%22category%22%3A%22button%22%7D) to
create one first.

1. In the [Auth0 dashboard](https://manage.auth0.com/#/), select
   **Applications**.

1. Select **Default App** and **Settings**.

1. Copy the **Domain** and then save the value as `AUTH0_DOMAIN` environment
   variable.

   **Example:**

   ```plaintext
   $ export AUTH0_DOMAIN=dev-2i513orw.auth0.com
   ```

1. Similarly, copy the **Client ID** and **Client Secret** values and save them
   as `AUTH0_CLIENT_ID` and `AUTH0_CLIENT_SECRET` environment variables.

   **Example:**

   ```shell
   # Set client ID
   $ export AUTH0_CLIENT_ID=FFXlsY2atr_aaaa_hMtsE-zTAeTZnu8

   # Set client secret
   $ export AUTH0_CLIENT_SECRET=1O7f9JQkv6b25Jf1m4io25h340rendfjfweoprjw-0-1284
   ```

1. In the **Allowed Callback URLs** field, enter the following:

   ```plaintext
   http://<consul_agent_address>:8550/oidc/callback
   http://<consul_agent_address>:4200/ui/torii/redirect.html
   ```

   The `http://<consul_agent_address>:8500/ui/torii/redirect.html` address will
   be used by the Consul UI when you login with OIDC auth method. The
   `http://<consul_agent_address>:8550/oidc/callback` address will be used by the
   CLI when you login via `consul login -type=oidc -method=oidc` command.

   **NOTE:** The callback URLs must be comma-separated.

   For example, if you are running your Consul agent _locally_ you would use:

   ```plaintext
   http://localhost:8550/oidc/callback,
   http://localhost:4200/ui/torii/redirect.html
   ```

## Step 2: Create Users with some metadata attached

1. In the Auth0 dashboard for your app, on the sidebar select **Users & Roles > Users**

1. Create at least one demo user with username/password authentication.

1. Edit the user's record and assign some `user_metadata` and `app_metadata` such as:

	```json
	{ // user_metadata (end-user editable)
		"first_name": "Kara",
		"last_name": "Danvers"
	}

	{ // app_metadata (end-user not editable)
		"roles": [ "engineering" ]
	}
	```

## Step 3: Configure claims in the ID Tokens

1. In the Auth0 dashboard for your app, on the sidebar select **Rules**

1. Create a new rule with the following content:

	```javascript
	function (user, context, callback) {
		user.user_metadata = user.user_metadata || {};
		user.app_metadata = user.app_metadata || {};

		context.idToken['http://consul.internal/first_name'] = user.user_metadata.first_name || "";
		context.idToken['http://consul.internal/last_name'] = user.user_metadata.last_name || "";
		context.idToken['http://consul.internal/int'] = -12345;
		context.idToken['http://consul.internal/float'] = 7.9;
		context.idToken['http://consul.internal/bool'] = true;

		context.idToken['http://consul.internal/groups'] = user.app_metadata.roles || [];
		callback(null, user, context);
	}
	```

## Step 4: Create policies and roles for the demo

    ```shell
    # create a policy called eng-ro to allow full read only access to service discovery
    $ CONSUL_HTTP_TOKEN=root \
        consul acl policy create -name eng-ro \
        -rules='service_prefix "" { policy="read" } node_prefix "" { policy="read" }'

    # create a policy called eng-ro that is linked to the similarly named policy
    $ CONSUL_HTTP_TOKEN=root \
        consul acl role create -name eng-ro -policy-name eng-ro
    ```

## Step 5: Enable the OIDC Auth Method

1. Create a new auth method.

	```shell
	$ cat > tmp-config.json <<EOF
	{
	  "Name": "auth0",
	  "Type": "oidc",
	  "MaxTokenTTL": "5m",
	  "Config": {
		"OIDCDiscoveryURL": "https://${AUTH0_DOMAIN}/",
		"OIDCClientID": "${AUTH0_CLIENT_ID}",
		"OIDCClientSecret": "${AUTH0_CLIENT_SECRET}",
		"BoundAudiences": [ "${AUTH0_CLIENT_ID}" ],
		"AllowedRedirectURIs": [
		  "http://localhost:8550/oidc/callback",
		  "http://localhost:4200/ui/torii/redirect.html"
		],
		"ClaimMappings": {
		  "http://consul.internal/int": "val_int",
		  "http://consul.internal/float": "val_float",
		  "http://consul.internal/bool": "val_bool",
		  "http://consul.internal/first_name": "first_name",
		  "http://consul.internal/last_name": "last_name"
		},
		"ListClaimMappings": {
		  "http://consul.internal/groups": "groups"
		}
	  }
	}
	EOF
	$ curl -sL -H 'x-consul-token: root' -XPUT \
		http://localhost:8500/v1/acl/auth-method \
		-d@tmp-config.json
    ```

1. Now configure some binding rules:

    ```shell
    # this binding rule will grant anyone in engineering the role 'eng-ro'
    $ CONSUL_HTTP_TOKEN=root \
        consul acl binding-rule create \
            -method=auth0 \
            -bind-type=role \
            -bind-name=eng-ro \
            -selector='engineering in list.groups'

    # this binding rule will grant anyone in engineering the ability to 
    # register a service usable in a service mesh with their own name
    $ CONSUL_HTTP_TOKEN=root \
        consul acl binding-rule create \
            -method=auth0 \
            -bind-type=service \
            -bind-name='dev-${value.first_name}-${value.last_name}' \
            -selector='engineering in list.groups'
    ```

## Step 6: Login with OIDC in the CLI

```shell
$ consul login -method=auth0 -type=oidc -token-sink-file=dev.token
```

When prompted, accept and authorize the Consul access to your Default App.

Your token secretID will be written to the `dev.token` sink file. It will look like:

    ```
    f110aff6-4d5b-4563-80dd-4da6f74c1067
    ```

You can check out the details of the token that was created via:

    ```shell
    $ consul acl token read -self -token-file=dev.token
    AccessorID:       cd887b52-263a-4ae2-8747-725b76d0a79f
    SecretID:         f110aff6-4d5b-4563-80dd-4da6f74c1067
    Namespace:        default
    Description:      token created via OIDC login
    Local:            true
    Auth Method:      auth0
    Create Time:      2020-04-28 15:00:49.790370772 -0500 CDT
    Roles:
       7d3e3a66-8c0f-e528-944e-84df6b36a115 - eng-ro
    Service Identities:
       dev-kara-danvers (Datacenters: all)
    ```
