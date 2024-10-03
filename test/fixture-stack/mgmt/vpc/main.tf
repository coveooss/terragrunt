variable "terraform_remote_state_s3_bucket" {
  description = "The name of the S3 bucket where Terraform remote state is stored"
}

terraform {
  backend "s3" {}
}

output "text" {
  value = "[I am a mgmt vpc template. I have no dependencies.]"
}
