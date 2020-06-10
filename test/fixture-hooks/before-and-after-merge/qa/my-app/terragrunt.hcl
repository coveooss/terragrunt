pre_hook "pre_hook_1" {
  on_commands = ["apply", "plan"]
  command     = "touch"
  arguments   = ["before.out"]
}

pre_hook "before_hook_merge_1" {
  on_commands = ["apply", "plan"]
  command     = "touch"
  arguments   = ["before-child.out"]
}

post_hook "post_hook_1" {
  on_commands = ["apply", "plan"]
  command     = "touch"
  arguments   = ["after.out"]
}

include {
  path = find_in_parent_folders()
}
