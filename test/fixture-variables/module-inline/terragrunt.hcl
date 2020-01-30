import_variables "test" {
  vars = [
    "hello=123",
  ]
}

export_variables {
  path = "test.tf"
}
