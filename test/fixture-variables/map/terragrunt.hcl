import_variables "test" {
  required_var_files = [
    "vars.json",
  ]
}

export_variables {
  path = "test.tf"
}
