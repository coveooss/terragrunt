terragrunt = {
  pre_hook "pre_hook_1" {
    on_commands = ["apply", "plan"]
    command     = "touch file.out"
  }
}
