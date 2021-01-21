import_variables "test" {
  vars = [
    "hello=123",
  ]
}

export_config {
  path = "test.tf"
}
