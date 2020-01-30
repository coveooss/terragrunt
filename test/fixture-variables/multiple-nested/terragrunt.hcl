import_variables "nested" {
  required_var_files    = ["vars.json"]
  nested_under          = ["main", "local", ""]
}

export_variables {
  path = "test.tf"
}
