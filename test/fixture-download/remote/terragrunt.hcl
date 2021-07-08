terraform {
  source = "github.com/coveooss/terragrunt.git//test/fixture-download/hello-world?ref=download_test"
}

inputs = {
  name = "remote"
}

export_variables {
  path = "terraform.tfvars"
}