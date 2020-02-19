terragrunt {
  import_variables "test" {
    required_var_files = [
      "vars.json",
    ]
    no_templating_in_files = true
  }
}

template = "123"
variables_file = "${save_variables("terraform.tfvars")}"