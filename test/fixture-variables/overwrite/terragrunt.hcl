import_variables "test" {
  vars = [
    "hello=123",
  ]

  output_variables_file = "test.tf"
}

import_variables "test2" {
  vars = [
    "hello=456",
  ]

  output_variables_file = "test.tf"
}