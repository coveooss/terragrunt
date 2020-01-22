terragrunt = {
  terraform {
    source = "${get_current_dir()}"
  }

  import_variables "test1" {
    vars = [
      "hello1=123",
    ]
  }

  import_variables "test2" {
    optional_var_files = [
      "vars1.json",
    ]
  }

  import_variables "test3" {
    required_var_files = [
      "vars2.json",
    ]
  }
}
