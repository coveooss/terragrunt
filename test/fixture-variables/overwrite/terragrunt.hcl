import_variables "test" {
  vars = [
    "hello=123",
  ]
}

import_variables "test2" {
  vars = [
    "hello=456",
  ]
}

export_variables {
  path = "test.tf"
}
