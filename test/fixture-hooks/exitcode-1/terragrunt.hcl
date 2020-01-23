post_hook "post_hook_1" {
  on_commands = ["apply", "plan"]
  command     = "exit 1"
}

post_hook "post_hook_2" {
  on_commands = ["apply", "plan"]
  command     = "touch test.out"
}