terragrunt = {
  import_variables "test" {
    required_var_files = [
      "vars1.json",
      "vars2.json",
    ]

    output_variables_file = "test.tf"
  }
}
