# Configure Terragrunt to automatically store tfstate files in an S3 bucket
remote_state {
  backend = "s3"

  config = {
    encrypt = true
    bucket  = "__FILL_IN_BUCKET_NAME__"
    key     = "${path_relative_to_include()}/terraform.tfstate"
    region  = "us-west-2"
  }
}

pre_hook "pre_hook_1" {
  on_commands = ["apply", "plan"]
  command     = "touch"
  arguments   = ["before.out"]
}

pre_hook "before_hook_merge_1" {
  on_commands = ["apply", "plan"]
  command     = "touch"
  arguments   = ["before-parent.out"]
}

post_hook "post_hook_1" {
  on_commands = ["apply", "plan"]
  command     = "touch"
  arguments   = ["after.out"]
}

post_hook "after_hook_parent_1" {
  on_commands = ["apply", "plan"]
  command     = "touch"
  arguments   = ["after-parent.out"]
}
