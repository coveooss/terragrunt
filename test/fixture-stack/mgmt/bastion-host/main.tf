terraform {
  backend "s3" {}
}

output "text" {
  value = "[I am a bastion-host template. Data from my dependencies: vpc = ${data.terraform_remote_state.vpc.outputs.text}]"
}

variable "terraform_remote_state_s3_bucket" {
  description = "The name of the S3 bucket where Terraform remote state is stored"
}

data "terraform_remote_state" "vpc" {
  backend = "s3"
  config = {
    region = "us-west-2"
    bucket = var.terraform_remote_state_s3_bucket
    key    = "mgmt/vpc/terraform.tfstate"
  }
}
