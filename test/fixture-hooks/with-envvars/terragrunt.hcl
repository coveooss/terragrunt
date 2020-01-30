extra_arguments "add-env-var" {
  display_name = "Add random environment variables"
  commands     = ["cmd1", "cmd2"]

  env_vars = {
    EXTRA_ARG1 = 1      # Environment variable are not required to be a string
    Extra_arg2 = "Arg2" # Environment name can also be in mixed case
  }
}

extra_command "cmd1" {
  env_vars = {
    EXTRA_COMMAND = "Extra command 1"
  }

  commands = ["echo Command 1"]
}

extra_command "cmd2" {
  env_vars = {
    EXTRA_COMMAND = "Extra command 2"
  }

  commands = ["echo Command 2"]
}

pre_hook "pr1" {
  on_commands = ["cmd1"]
  expand_args = true

  command = <<-EOF
  echo Hello from PreHook 1
  echo LOCAL_PRE_HOOK_1 = $LOCAL_PRE_HOOK_1
  echo GLOBAL_PRE_HOOK_1 = $GLOBAL_PRE_HOOK_1
  EOF

  env_vars = {
    LOCAL_PRE_HOOK_1 = "Pre-Hook #1"
  }

  persistent_env_vars = {
    GLOBAL_PRE_HOOK_1 = "Pre-Hook #1"
  }
}

before_hook "pr2" { # before_hook is simply an alias to pre_hook
  command = <<-EOF
  echo Hello from PreHook 2
  echo LOCAL_PRE_HOOK_2 = $LOCAL_PRE_HOOK_2
  echo GLOBAL_PRE_HOOK_2 = $GLOBAL_PRE_HOOK_2
  EOF

  on_commands = ["cmd2"]
  expand_args = true

  env_vars = {
    LOCAL_PRE_HOOK_2 = "Pre-Hook #2"
  }

  persistent_env_vars = {
    GLOBAL_PRE_HOOK_2 = "Pre-Hook #2"
  }
}

post_hook "po1" {
  command = <<-EOF
  echo Hello from PostHook 1
  echo LOCAL_POST_HOOK_1 = $LOCAL_POST_HOOK_1
  echo GLOBAL_POST_HOOK_1 = $GLOBAL_POST_HOOK_1
  EOF

  on_commands = ["cmd1"]
  expand_args = true

  env_vars = {
    LOCAL_POST_HOOK_1 = "Post-Hook #1"
  }

  persistent_env_vars = {
    GLOBAL_POST_HOOK_1 = "Post-Hook #1"
  }
}

after_hook "po2" { # after_hook is simple an alias for post_hook
  command = <<-EOF
  echo Hello from PostHook 2
  echo LOCAL_POST_HOOK_2 = $LOCAL_POST_HOOK_2
  echo GLOBAL_POST_HOOK_2 = $GLOBAL_POST_HOOK_2
  EOF

  on_commands = ["cmd2"]
  expand_args = true

  env_vars = {
    LOCAL_POST_HOOK_2 = "Post-Hook #2"
  }

  persistent_env_vars = {
    GLOBAL_POST_HOOK_2 = "Post-Hook #2"
  }
}

post_hook "end" {
  command = <<-EOF
  echo ----- Final environment -----
  env | grep -iE '(HOOK|EXTRA|IMPORTED)_' | sort
  EOF
}

import_variables "import" {
  env_vars = {
    IMPORTED_1 = "Imported #1"
    Imported_2 = "Imported #2"
  }

  on_commands = ["cmd1"]
}
