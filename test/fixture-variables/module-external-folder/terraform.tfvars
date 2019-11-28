terragrunt = {
  import_variables "test" {
    vars = [
      "my_variable=123",
    ]

    output_variables_file = "test.tf"
  }
}
