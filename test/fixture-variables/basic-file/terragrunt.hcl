import_variables "test" {
  required_var_files = [
    "vars.json",
  ]

  optional_var_files = [
    "not-exist.json",
  ]
}

export_variables {
  path = "test.tf"
}
