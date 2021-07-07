# This URL is intentionally invalid, as it should be overridden in the test case via command-line params
terraform {
  source = "invalid-url-should-be-overridden-at-test-time"
}

inputs = {
  name = "override"
}

export_variables {
  path = "terraform.tfvars"
}