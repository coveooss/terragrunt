terragrunt {
  pre_hook "hook_a" {
    on_commands = ["plan"]
    command     = "echo"
    arguments   = ["planHook"]
  }
}
