terragrunt = {
  import_variables "test" {
    vars = [
      "hello=123",
    ]

    output_variables_file = "test.tf"
  }

  post_hook "post_hook1" {
    on_commands = ["apply", "plan"]
    command     = "echo 'sub folder content:'"
  }

  post_hook "post_hook2" {
    on_commands = ["apply", "plan"]
    command     = "ls subfolder"
  }
}
