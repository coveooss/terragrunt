terragrunt {
  pre_hook "hook_b" {
    on_commands = ["apply"]
    command     = "echo"
    arguments   = ["applyHook"]
  }
}
