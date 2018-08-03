terragrunt = {
  pre_hook "pre_hook_1" {
    on_commands = ["apply", "plan"]
    command     = "echo ${get_env("HOME", "HelloWorld")}"
  }
}
