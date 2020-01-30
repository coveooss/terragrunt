import_variables "test" {
  sources            = ["my_source"]
  required_var_files = ["*.json"]
}

export_variables {
  path = "test.tf"
}
