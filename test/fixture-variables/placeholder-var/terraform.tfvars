terragrunt = {
  import_variables "test" {
    vars = [
      "hello",
    ]

    output_variables_file = "test.tf"
  }
}

hello = 123
