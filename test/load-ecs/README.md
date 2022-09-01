# Load Test Consul on ECS

Refer to `getting started` in the adjacent [load test for EC2 README.md](https://github.com/hashicorp/consul/blob/main/test/load/README.md)
to set up your environment for access to AWS. You can use keys, but in the example below, profiles are used.

The main difference to keep in mind, is this repository will source consul from this, your local, repository. Before you
begin using the makefile in this folder, ensure you have run `make dist` _or a similar command_ that will result in the
`consul:local` docker image being created and present in your docker image repository. 

> Ensure you have the consul:local container image built
> ```bash
> docker image ls | awk '{print $1":"$2}' | grep consul:local
> # consul:local
> ```

Then use `make` to:
- Create Consul TLS Certificates.
- Deploy base aws-infrastructure (network, iam, ecr, ecs-cluster, etc).
- Build consul/datadog/k6 images (based off your local `consul:local` image) and push to ECR.
- Deploy ECS Services with Task Definitions corresponding to the ECR images.
- Initiate a K6 Lambda Load Test.
- Collect information from CloudWatch Logs & Dashboards as well as Datadog's Consul Dashboard.
- Clean up the environment.

## This repo has the following folder structure

| Path         | Description                                                                                                                                                                                |
|--------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| containers   | Container assets for the development load-test stack                                                                                                                                       |
| modules      | Terraform modules used to simplify parts of the deployment.                                                                                                                                |
| tls          | An intentionally empty directory, used for generating a set of local Consul CA certificates. This will also be the directory, AWS SecretsManager source strings for the certs are sourced. |
 | templates    | Template used for terraform deployment                                                                                                                                                     |

> EFS can be disabled in the TF options. This is not recommended for use, but will allow comparisons to be made with and without the underlying EFS if desired.

## Commence the Tests

Your AWS environment will need to be authorized with the organization you wish to test within.

> __Warning__: This will incur AWS charges to the organization you are authorized!

You will also need to set environment variables to deliver a Consul Encryption Key.
The AWS Region, Datadog API Key and a K6 API Key are optional.

- TF_VAR_consul_encryption_token=[generate with `consul keygen`]
- TF_VAR_aws_region=[the region you intend to test in, default is us-east-1]
- TF_VAR_datadog_apikey=[generate in the datadog ui, optional]
- TF_VAR_k6_apikey=[generate in the k6 ui, optional]

Using environment variables here helps us ensure we don't save keys that can be compromised when we run this manually
and allows for workflows to set up an environment when running each step of the makefile. Consider the following command
to `make images` and `make infra`.
```text
AWS_PROFILE=test-3 TF_VAR_aws_region=us-east-2 make images

AWS_PROFILE=test-3 \
TF_VAR_consul_encryption_token=consul-encryption-token \
TF_VAR_datadog_apikey=DD_API_KEY \
TF_VAR_k6_apikey=k6_API_KEY \
make infra
```

Additionally, you will need an *.auto.tfvars file created similarly to the example provided. Note: 3 subnets are
__required__ in order to provide three sets of consul-server-agent combos.

```hcl
# Filename: ./variables.auto.tfvars
aws_region           = "us-east-2"
vpc_name             = "consul-test-vpc"
vpc_az               = ["us-east-2a", "us-east-2b", "us-east-2c"]
vpc_cidr             = "10.0.0.0/16"
public_subnet_cidrs  = ["10.0.1.0/24", "10.0.3.0/24", "10.0.5.0/24"]
private_subnet_cidrs = ["10.0.2.0/24", "10.0.4.0/24", "10.0.6.0/24"]

# deploy_efs_cluster = false

admin_cidrs = ["123.456.789.12/32"]
```

```bash
cd test/load-ecs/
cp variables.auto.tfvars.example variables.auto.tfvars
terraform init
make certs
AWS_PROFILE=test-3 TF_VAR_aws_region=us-east-2 make repos
AWS_PROFILE=test-3 TF_VAR_aws_region=us-east-2 make images
AWS_PROFILE=test-3 TF_VAR_aws_region=us-east-2 TF_VAR_consul_encryption_token=12345= TF_VAR_datadog_apikey=DDABC123 TF_VAR_k6_apikey=k6987ZYX make infra
AWS_PROFILE=test-3 TF_VAR_aws_region=us-east-2 TF_VAR_consul_encryption_token=12345= TF_VAR_datadog_apikey=DDABC123 TF_VAR_k6_apikey=k6987ZYX make test-ecs
```

When the test is invoked in the `make test-ecs` step above, you will be able to view the test output in the Lambda
Cloudwatch Logs associated with the k6 load test lambda function, running time ~10 minutes. There is also a CloudWatch
dashboard created for witnessing the ECS and EFS metrics.

The final output in CloudWatch will look something like the following, unfortunately this is not available as terraform
output:
```text
scenarios: (100.00%) 1 scenario, 25 max VUs, 10m30s max duration (incl. graceful stop):
...
default [ 15% ] 25 VUs 01m28.9s/10m0s
running (01m29.9s), 25/25 VUs, 2859 complete and 0 interrupted iterations
...
default [ 61% ] 25 VUs 06m07.9s/10m0s
time="2022-07-21T13:38:46Z" level=warning msg="Request Failed" error="Put \"https://internal-consul-examplelb-1234567890.us-east-2.elb.amazonaws.com:8500/v1/agent/service/register\": request timeout"
...
running (10m01.4s), 00/25 VUs, 11041 complete and 0 interrupted iterations
default ✓ [ 100% ] 25 VUs 10m0s
↳ 99% — ✓ 11017 / ✗ 24
checks.......................: 99.89% ✓ 22058 ✗ 24
✓ http_req_duration..........: avg=679.57ms min=29.36ms med=112.67ms max=1m0s p(90)=1.44s p(95)=1.92s
{ expected_response:true }...: avg=615.03ms min=29.36ms med=112.46ms max=55.78s p(90)=1.43s p(95)=1.9s

```

The test takes about 10 minutes to run. Don't forget to clean-up the environment :)

```bash
AWS_PROFILE=test-3 make clean
```

## Considerations

- This is not a production-ready stack. While it implements gossip encryption and agent communication tls, there are
  many aspects not considered for the purpose of creating a testable proof-of-concept in ECS.

- The ALB is internal by default. If you set it to external and define an ingress admin ip range, you can connect to the
  consul servers on port 18500 and the clients on 8500. You will not be able to run the lambda load test, but this can
  be useful for debugging. Traffic over the public interface can get expensive.

- Each consul service and agent combo share an availability zone (ie: 3 zones, one-server + one-client per zone). Since
  there is only one k6-lambda, that can potentially be in any of the three subnets, there is additional network load 
  placed on one particular availability-zone as opposed to the others.

- Fargate does not allow setting the DNS Server of a container task and resolv.conf is bind mounted ro. Therefore, no dns
  queries are made to consul in the scope of this test. The EFS mounts are mounted via Route53 dns names, so vpc dns is
  enabled.

- This load test is not automated, and it has not been tested in a workflow, but could be following the same method as
  the [load-test.yml workflow](https://github.com/hashicorp/consul/blob/main/.github/workflows/load-test.yml)
