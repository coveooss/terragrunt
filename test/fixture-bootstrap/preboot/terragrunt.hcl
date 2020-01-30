pre_hook "print_variable" {
  on_commands = ["apply"]
  command     = "echo"
  arguments   = ["@(my_variable)", "@(my_variable2)", "@(my_variable3)"]
}
