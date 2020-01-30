terragrunt {
  import_variables "not-flatten" {
    required_var_files    = ["vars.json"]
    nested_under          = ["main", "local", ""]
    output_variables_file = "not-flatten.tf"
  }

  import_variables "flatten" {
    required_var_files    = ["vars.json"]
    nested_under          = ["main", "local", ""]
    flatten_levels        = 2
    output_variables_file = "flatten.tf"
  }
}
