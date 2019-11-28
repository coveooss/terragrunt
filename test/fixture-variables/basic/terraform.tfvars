terragrunt = {
  import_variables "test" {
    vars = [
      "hello=123",
    ]

    output_variables_file = "test.tf"
  }

  post_hook "subfolder_ls" {
    on_commands = ["apply", "plan"]
    command     = "echo \"sub folder content: $(ls subfolder)\""
  }
}
