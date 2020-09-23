import_variables "test" {
  required_var_files = ["value.hcl"]
  nested_under       = ["alias", ""]
}

export_variables {
  path = "test.tf"
}
