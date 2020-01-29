terragrunt = {
  import_variables "test" {
    required_var_files = [
      "vars.tf",
    ]

    nested_under          = ["loaded"]
    output_variables_file = "test.tf"
  }
}
