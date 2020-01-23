pre_hook "print_variable" {
  on_commands = ["apply"]
  command     = "echo"
  arguments   = ["@(key1)", "@(key2)"]
}