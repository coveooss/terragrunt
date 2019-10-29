terragrunt = {
  import_variables "test" {
    sources            = ["my_source"]
    required_var_files = ["*.json"]

    output_variables_file = "test.tf"
  }
}
