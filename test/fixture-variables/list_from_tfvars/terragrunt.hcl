import_variables "test" {
  required_var_files = [
    "test.tfvars",
  ]
}

export_variables {
  path = "terraform.tfvars"
}
