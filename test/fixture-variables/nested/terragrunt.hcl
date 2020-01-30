import_variables "test2" {
  vars = [
    "var=123",
  ]

  nested_under = ["nested1"]
}

import_variables "test" {
  optional_var_files = [
    "vars.json",
  ]

  nested_under = ["nested2"]
}

export_variables {
  path = "test.tf"
}
