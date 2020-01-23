import_variables "test" {
  required_var_files = [
    "*.json",
  ]

  optional_var_files = [
    "*.yml",
  ]
}

export_variables {
  path = "test.tf"
}
