import_variables "test" {
  required_var_files = [
    "vars.json",
  ]
  no_templating_in_files = true
}

inputs = {
  template = "123"
}

export_variables {
  path = "test.tf"
}
