terraform {
  backend "s3" {}
}

# Create an arbitrary local resource
locals {
  test = "Hello, I am a template. My sample_var value = $${sample_var}"
}

output "rendered_template" {
  value = templatestring(local.test, {sample_var = var.sample_var})
}

# Configure these variables
variable "sample_var" {
  description = "A sample_var to pass to the template."
  default     = "hello"
}
