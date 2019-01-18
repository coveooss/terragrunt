terragrunt = {
  terraform {
    source = "${get_current_dir()}"
  }

  import_variables "test" {
    vars = [
      "hello1=123",
    ]
  }

  import_variables "test" {
    optional_var_files = [
      "vars1.json",
    ]
  }

  import_variables "test" {
    required_var_files = [
      "vars2.json",
    ]
  }
}
