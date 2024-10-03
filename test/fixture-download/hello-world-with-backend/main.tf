variable "name" {
  description = "Specify a name"
}

output "example" {
  value = "hello, ${var.name}"
}

terraform {
  # These settings will be filled in by Terragrunt
  backend "s3" {}
}
