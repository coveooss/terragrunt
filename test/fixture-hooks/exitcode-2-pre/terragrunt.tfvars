terragrunt = {
  pre_hook "pre_hook_1" {
    on_commands = ["apply", "plan"]
    command     = "exit 2"
  }

  post_hook "post_hook_1" {
    on_commands = ["apply", "plan"]
    command     = "touch test2.out"
  }
}
