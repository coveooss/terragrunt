#! @{my_variable} := "test variable"

terragrunt {
  pre_hook "print_variable" {
    command   = "echo"
    arguments = ["@{my_variable}"]
  }
}
