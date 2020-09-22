provider "aws" {
  alias  = "test"
  region = "eu-west-1"
}

resource "aws_security_group" "test" {
  provider = "aws.@(`test`)"
  count    = 0
}

output "ok" {
  value = "Everything is fine"
}
