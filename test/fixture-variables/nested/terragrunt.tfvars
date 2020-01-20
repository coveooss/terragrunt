terragrunt = {
  import_variables "test2" {
    vars = [
      "var=123",
    ]

    nested_under = ["nested1"]

    output_variables_file = "test.tf"
  }

  import_variables "test" {
    optional_var_files = [
      "vars.json",
    ]

    nested_under = ["nested2"]

    output_variables_file = "test.tf"
  }
}
