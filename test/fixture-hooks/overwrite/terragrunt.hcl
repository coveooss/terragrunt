extra_command "cmd" {
  commands = ["echo Completed"]
}

pre_hook "hook" {
  command = "echo Pre-hook"
}

post_hook "hook" {
  command = "echo Post-hook"
}

pre_hook "hook" {
  command = "echo Pre-hook redefined"
}