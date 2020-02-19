import_variables "test" {
  required_var_files = [
    "vars.json",
  ]
}

inputs = {
  template = "123"
}

export_variables {
  path = "test.tf"
}
