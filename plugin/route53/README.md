# route53

## Name

*route53* - enables serving zone data from AWS route53.

## Description

The route53 plugin is useful for serving zones from resource record sets in AWS route53.
This plugin only supports A and AAAA records. The route53 plugin can be used when
coredns is deployed on AWS.

## Syntax

~~~ txt
route53 [ZONE:HOSTED_ZONE_ID...] {
    [aws_access_key AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY]
}
~~~

* **ZONE** the name of the domain to be accessed.
* **HOSTED_ZONE_ID** the ID of the hosted zone that contains the resource record sets to be accessed.
* **AWS_ACCESS_KEY_ID** and **AWS_SECRET_ACCESS_KEY** the AWS access key ID and secret access key
   to be used when query AWS (optional).  If they are not provided, then coredns tries to access
   AWS credentials the same way as AWS CLI, e.g., environmental variables, AWS credentials file,
   instance profile credentials, etc.

## Examples

Enable route53, with implicit aws credentials:

~~~ txt
. {
    route53 example.org.:Z1Z2Z3Z4DZ5Z6Z7
}
~~~

Enable route53, with explicit aws credentials:

~~~ txt
. {
    route53 example.org.:Z1Z2Z3Z4DZ5Z6Z7 {
      aws_access_key AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY
  }
}
~~~
