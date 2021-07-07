terraform {
  source = "../hello-world"
}

inputs = {
  name = "local"
}

export_variables {
  path = "terraform.tfvars"
}