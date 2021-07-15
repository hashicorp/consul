# startLoadTest Lambda Function

startLoadTest is an AWS lambda function that responds to events containing a deployment string. Currently there are two deployments accepted, `"load-test", "pull-request"`.
    - `"load-test"` will create a Consul cluster, ELB, and ec2 instances w/ k6. Then it will configure and run the K6 processes to apply traffic to the Consul cluster.
    - `"pull-request"` is not yet implemented.
    
startLoadTest uses a job queue via SQS to begin jobs and accept input. In the future, we intend on generating job events from a slack bot and webhooks created from Pull Requests. Anything that can generate an HTTP request to issue a job event on SQS will be capable of issuing jobs to startLoadTest workers, which are spawned as-needed via AWS Lambda.

## To Deploy
- Build the binary with `env GOARCH=amd64 GOOS=linux go build startLoadTest.go`
- Zip the build artifact with `zip startLoadTest.zip startLoadTest`
- Upload the zip containing the binary to https://console.aws.amazon.com/lambda/home?region=us-east-1#/functions/startLoadTest/versions/$LATEST

## To Run
- TODO
