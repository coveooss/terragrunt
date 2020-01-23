pre_hook "pre_hook_1" {
  on_commands = ["apply", "plan"]
  command     = "touch"
  arguments   = ["before.out"]
}

post_hook "post_hook_1" {
  on_commands = ["apply", "plan"]
  command     = "touch"
  arguments   = ["after.out"]
}