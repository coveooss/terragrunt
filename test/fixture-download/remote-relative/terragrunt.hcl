terraform {
  source = "github.com/coveooss/terragrunt.git//test/fixture-download/relative?ref=download_test"
}

inputs = {
  name = "remote-relative"
}

export_variables {
  path = "terraform.tfvars"
}