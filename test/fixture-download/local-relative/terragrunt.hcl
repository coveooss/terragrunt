terraform {
  source = "..//relative"
}

inputs = {
  name = "local-relative"
}

export_variables {
  path = "terraform.tfvars"
}