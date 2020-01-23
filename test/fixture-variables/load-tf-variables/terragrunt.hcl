import_variables "test" {
  required_var_files = [
    "vars.tf",
  ]

  nested_under = ["loaded"]
}

// Only variables that aren't already in vars.tf should be exported, otherwise there will be conflicts
export_variables {
  path = "test.tf"
}
