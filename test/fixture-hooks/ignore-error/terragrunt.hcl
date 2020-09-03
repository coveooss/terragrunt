pre_hook "pre" {
  on_commands  = ["apply", "plan"]
  command      = "exit 1"
  ignore_error = true
}

post_hook "post" {
  on_commands  = ["apply", "plan"]
  command      = "exit 1"
  ignore_error = true
}