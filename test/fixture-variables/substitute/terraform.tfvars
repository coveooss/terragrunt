terragrunt = {
  import_variables "test1" {
    vars = [
      "var1=hello",
    ]

    output_variables_file = "test.tf"
  }

  import_variables "test2" {
    vars = [
      "var2=${var.var1}2",
    ]

    output_variables_file = "test.tf"
  }
}
