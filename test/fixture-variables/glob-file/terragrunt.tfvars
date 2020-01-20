terragrunt = {
  import_variables "test" {
    required_var_files = [
      "*.json",
    ]

    optional_var_files = [
      "*.yml",
    ]

    output_variables_file = "test.tf"
  }
}
