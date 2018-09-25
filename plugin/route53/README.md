# route53

## Name

*route53* - enables serving zone data from AWS route53.

## Description

The route53 plugin is useful for serving zones from resource record sets in AWS route53. This plugin
supports all Amazon Route 53 records (https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/ResourceRecordTypes.html).
The route53 plugin can be used when coredns is deployed on AWS or elsewhere.

## Syntax

~~~ txt
route53 [ZONE:HOSTED_ZONE_ID...] {
    [aws_access_key AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY]
    upstream [ADDRESS...]
    credentials PROFILE [FILENAME]
    fallthrough [ZONES...]
}
~~~

* **ZONE** the name of the domain to be accessed.
* **HOSTED_ZONE_ID** the ID of the hosted zone that contains the resource record sets to be accessed.
* **AWS_ACCESS_KEY_ID** and **AWS_SECRET_ACCESS_KEY** the AWS access key ID and secret access key
   to be used when query AWS (optional).  If they are not provided, then coredns tries to access
   AWS credentials the same way as AWS CLI, e.g., environmental variables, AWS credentials file,
   instance profile credentials, etc.
* `upstream` [**ADDRESS**...] specifies upstream resolver(s) used for resolving services that point
   to external hosts (eg. used to resolve CNAMEs). If no **ADDRESS** is given, CoreDNS will resolve
   against itself. **ADDRESS** can be an IP, an IP:port or a path to a file structured like
   resolv.conf (**NB**: Currently a bug (#2099) is preventing the use of self-resolver).
* `credentials` used for reading the credential file and setting the profile name for a given zone.
* **PROFILE** AWS account profile name. Defaults to `default`.
* **FILENAME** AWS credentials filename. Defaults to `~/.aws/credentials`
   are used.
* `fallthrough` If zone matches and no record can be generated, pass request to the next plugin.
  If **[ZONES...]** is omitted, then fallthrough happens for all zones for which the plugin
  is authoritative. If specific zones are listed (for example `in-addr.arpa` and `ip6.arpa`), then only
  queries for those zones will be subject to fallthrough.
* **ZONES** zones it should be authoritative for. If empty, the zones from the configuration block

## Examples

Enable route53 with implicit AWS credentials and an upstream:

~~~ txt
. {
    route53 example.org.:Z1Z2Z3Z4DZ5Z6Z7
    upstream 10.0.0.1
}
~~~

Enable route53 with explicit AWS credentials:

~~~ txt
. {
    route53 example.org.:Z1Z2Z3Z4DZ5Z6Z7 {
      aws_access_key AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY
    }
}
~~~

Enable route53 with fallthrough:

~~~ txt
. {
    route53 example.org.:Z1Z2Z3Z4DZ5Z6Z7 example.gov.:Z654321543245 {
      fallthrough example.gov.
    }
}
~~~

Enable route53 with AWS credentials file:

~~~ txt
. {
    route53 example.org.:Z1Z2Z3Z4DZ5Z6Z7 {
      credentials_file some-user
    }
}
~~~
