terragrunt = {
  import_variables "test2" {
    vars = [
      "hello=123",
    ]

    output_variables_file = "test.tf"
  }

  import_variables "test" {
    optional_var_files = [
      "vars.json",
    ]

    output_variables_file = "test.tf"
  }
}
