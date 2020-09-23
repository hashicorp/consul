## Load Test AMI
This AMI will be used for all load test servers. Currently it copies the `/scripts` and installs Locust, so if any additional files are desired place them in that directory.

# How to use
1) Set the AWS region in the `loadtet.json` file
2) Run the command `packer build loadtest.json` 