# Terragrunt

[![Maintained by Gruntwork.io](https://img.shields.io/badge/maintained%20by-gruntwork.io-%235849a6.svg)](https://gruntwork.io/?ref=repo_terragrunt)
[![Go Report Card](https://goreportcard.com/badge/github.com/gruntwork-io/terragrunt)](https://goreportcard.com/report/github.com/gruntwork-io/terragrunt)
[![GoDoc](https://godoc.org/github.com/gruntwork-io/terragrunt?status.svg)](https://godoc.org/github.com/gruntwork-io/terragrunt)
![Terraform Version](https://img.shields.io/badge/tf-%3E%3D0.12.0-blue.svg)

Terragrunt is a thin wrapper for [Terraform](https://www.terraform.io/) that provides extra tools for keeping your
Terraform configurations [DRY](https://en.wikipedia.org/wiki/Don%27t_repeat_yourself),
working with multiple Terraform modules, and managing remote state.

Please see the following for more info, including install instructions and complete documentation:

* [Terragrunt Website](https://terragrunt.gruntwork.io)
* [Getting started with Terragrunt](https://terragrunt.gruntwork.io/docs/getting-started/quick-start/)
* [Terragrunt Documentation](https://terragrunt.gruntwork.io/docs)
* [Contributing to Terragrunt](http://terragrunt.gruntwork.io/docs/community/contributing)

## Additional features in this fork

### Configuration file

Terragrunt supports defining configuration file as `terragrunt.hcl` for pure hcl configuration or `terragrunt.hcl.json` if you want to express your
configuration as json.

It is also possible to name your file with different name (letting you organize your folder as you want and not mix the terragrunt configuration file
with your regular terraform files).

Supported names are:

* `.terragrunt`, `terragrunt`, `.terragrunt.config`, `terragrunt.config` (can use hcl, json or yaml syntax)
* `.terragrunt.hcl` or `terragrunt.hcl` (must use hcl syntax)
* `.terragrunt.json`, `terragrunt.json`, `.terragrunt.hcl.json` or `terragrunt.hcl.json` (must use json syntax)
* `.terragrunt.yaml`, `terragrunt.yaml`, `.terragrunt.yml` or `terragrunt.yml` (must use yaml syntax)

It is also possible to add your own terragrunt config file name by specifying the `--terragrunt-config` argument or by defining the environment
variable `TERRAGRUNT_CONFIG`. If this argument point on a specific path, the working directory will be set to the containing folder. If it
is just a filename (without folder), then, it will be joined to the specified working directory (current folder being the default). Only one
file can be specified.

```bash
terragrunt --terragrunt-config my-custom-config
```

Then, the filename `my-custom-config` will also be accepted as a valid terragrunt configuration name.

### Assume AWS IAM role

Terraform already provides the functionality to configure AWS provider that assume a different IAM Role when retrieving and creating AWS
resources. But when we use terragrunt to configure S3 backend to store our remote states, terraform uses the current user rights to
access and configure the remote state file and to manage locking operation in the DynamoDB database.

Since the state files may contain secrets, it is often required to restrict access to these files. But event if the AWS provider is
configured to allow access to the state file by assuming a role, the call will fail if the current user does not have a direct access to
theses files.

Moreover, if the user has configured its AWS profile (in .aws/config) to assume a role instead of directly using credentials, terraform
would not be able to recognize that configuration and will complain that there is `No valid credential sources found for AWS Provider`

```text
[profile deploy]
source_profile = default
role_arn = arn:aws:iam::9999999999999:role/deploy-role
region = us-east-1
```

#### Configure role

To solve that problem, it is possible to tell terragrunt to assume a different IAM role when it calls terraform operations.

```hcl
assume_role = "arn:aws:iam::9999999999999:role/deploy-terraform-role"
```

That also could be defined as an array of roles:

```hcl
assume_role = [
  "arn:aws:iam::9999999999999:role/read-write",
  "arn:aws:iam::9999999999999:role/read-only",
  "",
]
```

Then, terragrunt will try to assume the roles until it succeeded. Note that the last role could be optionally set to an empty string to
ensure that at least one role is satisfied. Empty strings means to continue with the user current role.

The `assume_role` configuration could be defined in any terragrunt configuration files. If it is defined at several level, the leaf
configuration will prevail.

### Conditional execution of a project

It is possible to set conditions that must be met in order for a project to be executed. To do so, the following block must be defined in the terragrunt configuration file:

* All conditions within a `condition` attribute must be true to apply the rule (logical and between elements).  
* If any block marked `ignore_if_true` is true, the code is not executed (logical or between elements).  
* If any block *NOT* marked `ignore_if_true` is true, the code is executed (logical or between element).  
* *Important: `ignore_if_true` blocks always take precedence over those that aren't*  

```hcl
run_conditions {
  run_if = {
    region = ["us-east-1", "us-west-2"] # region must match one of "us-east-1" or "us-west-2"
    another_var = "value"               # another_var must be equal to "value"
    my_map.my_var = "value"             # the `my_var` item of the `my_var` map must be equal to "value"
  }
}

run_conditions {
  ignore_if = {
    env = "qa"                          # do not run if env = "qa"
  }
}
```

The condition for this project to run would be as follows:  
`region in ["us-east-1", "us-west-2"] AND another_var in ["value"] AND my_map.my_var in ["value"] AND env not in ["qa"]`

The conditions are evaluated using variables passed to terragrunt/terraform and the accepted or rejected values must be in the form of a list.  
Any non existing variable will cause an error and the project will be ignored.  
For more complex situation, it is possible to use variable interpolation as the key:  

```hcl
run_conditions {
  run_if = {
    "${var.env}/${var.region}" = ["dev/us-east-1", "qa/us-west-2"]  # env/region must match one of "dev/us-east-1" or "qa/us-west-2"
  }
}
```

### Define extra commands

Since Terragrunt configure the execution context in temporary folder, it may be useful to execute other command than terraform in that context after
the terraform remote state has been configured.

#### Configure extra commands

```hcl
extra_command "name" {
  description = ""                  # Description of the extra command action
  command     = ""                  # optional (default use name as the command, actual command to use instead of name)
  commands    = [list of commands]  # optional (list of other commands that should be included and are having the same behavior)
  aliases     = [list of alias]     # optional (list of alias to command or name or first item of commands)
  os          = [list of os]        # optional (default run on all os, os name are those supported by go, i.e. linux, darwin, windows)
  use_state   = true or false       # optional (default = true)
  act_as      = "command"           # optional (default = empty, instructs to consider this extra command and its aliases as another command regarding extra_parameters evaluation)
  version     = ""                  # optional (argument to get the version of the command, if many command are defined, they must all support the same argument to get the version)
  env_vars    = {}                  # optional (define environment variables only available during hook execution)
}
```

#### Example of extra commands

```hcl
  # Add extra commands to terragrunt
  extra_command "shell" {
    commands = ["bash", "sh", "zsh", "fish", "ls"]
    os       = ["darwin", "linux"]
  }
```

So the following commands do:

starts a shell into the temporary folder

```bash
> terragrunt bash
> terragrunt sh
> terragrunt zsh
> terragrunt fish
> terragrunt shell
```

List the content of the temporary folder

```bash
> terragrunt ls -al
```

The name `shell` used to name the extra_command group could also be used as a command. It acts as an alias for the first command in
`commands` list.

### Define hooks

It may be useful to define some additional commands that should be ran before and after executing the actual terraform command. You can
define hooks in any terragrunt configuration blocks. By default, pre hooks are executed the declaration order starting with hooks defined
in the uppermost terragrunt configuration block (parents) and finishing with those defined in the leaf configuration block. You can alter
the execution order by specifying a different order (non specified order are set to 0 by default).

It is possible to override hooks defined in a parent configuration by specifying the exact same name.

#### Configure hooks

```hcl
pre_hook|post_hook "name" {
  description      = ""                             # Description of the hook
  command          = "command"                      # shell command to execute
  on_commands      = [list of terraform commands]   # optional, default run on all terraform command
  os               = [list of os]                   # optional, default run on all os, os name are those supported by go, i.e. linux, darwin, windows
  arguments        = [list of arguments]            # optional
  expand_args      = false                          # optional, expand pattern like *, ? [] on arguments
  ignore_error     = false                          # optional, continue execution on error
  before_imports   = false                          # optional, run command before terraform imports its files
  after_init_state = false                          # optional, run command after the state has been initialized
  order            = 0                              # optional, default run hooks in declaration order (hooks defined in uppermost parent first, negative number are supported)
  env_vars         = {}                             # optional, define environment variables only available during hook execution
  global_vars      = {}                             # optional, define global environment variables (exist while and after executing the hook)
}
```

#### Example of hook

```hcl
  # Do terraform get before plan
  pre_hook "get-before-plan" {
    command          = "terraform"
    arguments        = ["get"]
    on_commands      = ["plan"]
    after_init_state = true

    env_vars = {
      VAR1 = "Value 1"
      VAR2 = 1234
    }

    global_vars = {
      GLOBAL1 = "Global Value 1"
      GLOBAL2 = 1234
    }
  }

  # Print the outputs as json after successful apply
  post_hook "print-json-output" {
    command          = "terraform"
    arguments        = ["output", "-json"]
    on_commands      = ["apply"]
  }
```

### Import variables

It is possible to import variables from external files (local or remote) or defined variables directly in a `import_variables`
configuration block. It is also possible to define environment variables that will be defined during the whole terragrunt command
execution.

#### Configure import variables

```hcl
import_variables "name" {
  description            = ""            # Description of the import variables action
  display_name           = ""            # The name used in documentation (default to block name)
  sources                = ["path"]      # Specify the sources of the copy (currently only support S3 sources)
  vars                   = []            # Optional, array of key=value statements to define variables
  required_var_files     = []            # Optional, array of file names containing variables that should be imported
  optional_var_files     = []            # Optional, same as required_var_files but does not report error if the file does not exist
  env_vars               = {}            # optional, define environment variables only available during hook execution
  nester_under           = []            # Optional, define variables under a specific object
  os                     = [list of os]  # Optional, default run on all os, os name are those supported by go, i.e. linux, darwin, windows
  disabled               = false         # Optional, provide a mechanism to temporary disable the import variables block
}
```

#### Example of import variables

```hcl
  import_files "global-variables" {
    source              = "s3://my_bucket-${var.env}/globals"
    required_var_files  = ["account.tfvars"]
    optional_var_files  = ["optional.tfvars", "optional2.tfvars"]
    vars = [
      "a=1",
      "b=hello",
    ]
    env_vars = {
      VAR1 = "Value 1"
      VAR2 = "Value 2"
    }
  }
```

### Import files

When terragrunt execute, it creates a temporary folder containing the source of your terraform project and the configuration file.
It is also possible to import files from external sources that should be used by terraform to evaluate your project.
One typical usage of this feature is to import global variables that are common to all your terraform projects.

#### Configure import files

```hcl
import_files "name" {
  description         = ""                            # Description of the import files action
  source              = "path"                        # Specify the source of the copy (currently only support [S3 sources](https://www.terraform.io/docs/modules/sources.html).))
  files               = ["*"]                         # Optional, copy all files matching one of the pattern
  copy_and_rename     = []                            # Optional, specific rule to copy file from the source and rename it in the target
  required            = false                         # Optional, generate an error if there is no matching file
  import_into_modules = false                         # Optional, specify to apply the import files also on each module subfolder
  file_mode           = mode                          # Optional, typically octal number such as 0755
  target              = ""                            # Optional, default is current temporary folder
  prefix              = ""                            # By default, imported file are prefixed by the "name" of the import rule, can be overridden by specifying a prefix
  os                  = [list of os]                  # optional, default run on all os, os name are those supported by go, i.e. linux, darwin, windows
}
```

#### Example of import files

```hcl
  import_files "global-variables" {
    source              = "s3://my_bucket-${var.env}/globals"
    patterns            = ["*.tf", "*.tf.json", "*.tfvars"]
    import_into_modules = true
  }

  # Rename file from the source to a specific name
  import_files "various-file" {
    source              = "s3://my_bucket-${var.env}/various.zip"
    prefix              = ""
    copy_and_rename = [
      {
        source = "source_file"
        target = "target_file"
      }
    ]
  }
```

### Uniqueness criteria

When terragrunt execute, it creates a temporary folder containing the source of your terraform project and the configuration file. It is
also possible to import files from external sources that should be used by terraform to evaluate your project. One typical usage of this
feature is to import global variables that are common to all your terraform projects.

#### Configure uniqueness criteria

Terragrunt create may temporary folders for each of your terraform project based on the path where your terraform source are located. But
if you change variables such as your environment, your deployment region or your project name. You may face get conflicts between your
current cached temporary folder and your execution context. To ensure that all your different environments get a distinct temporary
folder, you may define a `uniqueness_criteria`. That criteria will be added to the source folder to generate a unique and distinct
temporary folder name.

```hcl
uniqueness_criteria = "${var.env}${var.region}/${var.project}"
```

### Export variables to a file

There are various ways to import variables such as `inputs` in the terragrunt config or `import_variables` blocks but these variables are
not accessible by Terraform directly

To write your imported variables to a file, use the `export_variables` block. Example:  

```hcl
export_variables {
  format = "tf" (Accepted values: "tfvars", "yaml", "json", "tf", "hcl")
  path   = "path_to_file.tf"
}
```

### set_global_variable

`set_global_variable(key, value)` allows users to add/modify a global variable. Example:

```hcl
# @set_global_variable("Today", now().Weekday())                     // Used as Razor function
# {{ set_global_variable "Tomorrow" (now.AddDate 0 0 1).Weekday }}   // Used as go template function
```

## License

This code is released under the MIT License. See [LICENSE.txt](LICENSE.txt).
