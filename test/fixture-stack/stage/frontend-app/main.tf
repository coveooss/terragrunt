terraform {
  backend "s3" {}
}

output "text" {
  value = "[I am a frontend-app template. Data from my dependencies: vpc = ${data.terraform_remote_state.vpc.outputs.text}, bastion-host = ${data.terraform_remote_state.bastion_host.outputs.text}, backend-app = ${data.terraform_remote_state.backend_app.outputs.text}]"
}

variable "terraform_remote_state_s3_bucket" {
  description = "The name of the S3 bucket where Terraform remote state is stored"
}

data "terraform_remote_state" "vpc" {
  backend = "s3"
  config = {
    region = "us-west-2"
    bucket = var.terraform_remote_state_s3_bucket
    key    = "stage/vpc/terraform.tfstate"
  }
}

data "terraform_remote_state" "backend_app" {
  backend = "s3"
  config = {
    region = "us-west-2"
    bucket = var.terraform_remote_state_s3_bucket
    key    = "stage/backend-app/terraform.tfstate"
  }
}


data "terraform_remote_state" "bastion_host" {
  backend = "s3"
  config = {
    region = "us-west-2"
    bucket = var.terraform_remote_state_s3_bucket
    key    = "mgmt/bastion-host/terraform.tfstate"
  }
}
