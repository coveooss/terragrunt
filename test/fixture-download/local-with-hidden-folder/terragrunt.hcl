terraform {
  source = "../hello-world"
}

inputs = {
  name = "local-with-hidden-folder"
}

export_variables {
  path = "terraform.tfvars"
}