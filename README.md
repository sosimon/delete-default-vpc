# delete-default-vpc

Delete default VPC in all regions in an AWS account.

There are Python and Bash scripts that do this like [this](https://github.com/davidobrien1985/delete-aws-default-vpc) and [this](https://gist.github.com/jokeru/e4a25bbd95080cfd00edf1fa67b06996), but Python requires Python to be installed, and Bash is quite slow.

## Requirements

* `go >= 1.15.2`
* AWS credentials with appropriate permissions to delete VPC, IGW, and Subnets


## Build

`go build -o delete-default-vpc`

## Run

`./delete-default-vpc`
