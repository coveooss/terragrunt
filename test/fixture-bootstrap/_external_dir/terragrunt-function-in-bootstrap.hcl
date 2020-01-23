pre_hook "print_env" {
  command   = "echo"
  arguments = ["@get_env("TEST", "default_env_value")"]
}
