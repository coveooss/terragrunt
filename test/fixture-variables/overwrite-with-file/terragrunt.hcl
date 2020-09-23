import_variables "test2" {
  vars = [
    "not_overwritten=stay the same",
    "value_set_in_file",
  ]
}

// Variables defined in files have a lower priority than those defined explicitly
import_variables "test" {
  optional_var_files = ["vars.json"]
}

export_variables {
  path = "test.tf"
}
