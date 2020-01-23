import_variables "test" {
  vars = [
    "hello=123",
  ]
}

post_hook "subfolder_ls" {
  on_commands = ["apply", "plan"]
  command     = "echo \"sub folder content: $(ls subfolder)\""
}

export_variables {
  path = "test.tf"
}
