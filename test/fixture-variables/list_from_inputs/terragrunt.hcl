export_variables {
  path = "terraform.tfvars"
}

inputs = {
  my_list = [
    {
      var1 = "value5"
      var2 = "value6"
    }
  ]
}
