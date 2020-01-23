#! @{my_variable} := "test variable"

pre_hook "print_variable" {
  command   = "echo"
  arguments = ["@{my_variable}"]
}
