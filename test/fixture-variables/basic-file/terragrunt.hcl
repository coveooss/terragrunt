terragrunt = {
  import_variables "test" {
    required_var_files = [
      "vars.json",
    ]

    optional_var_files = [
      "not-exist.json",
    ]

    output_variables_file = "test.tf"
  }
}
