terragrunt = {
  post_hook "post_hook_1" {
    on_commands = ["apply", "plan"]
    command     = "touch file.out"
  }
}
