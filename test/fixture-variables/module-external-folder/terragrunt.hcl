import_variables "test" {
  vars = [
    "my_variable=123",
  ]
}

export_variables {
  path = "test.tf"
}
