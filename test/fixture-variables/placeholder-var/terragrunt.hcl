import_variables "test" {
  vars = [
    "hello",
  ]

  output_variables_file = "test.tf"
}

inputs = {
  hello              = 123
  variables_location = "${save_variables("terraform.tfvars")}"
}
