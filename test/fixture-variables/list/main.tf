variable "my_list" {

  type = list(object({
    var1 = string
    var2 = string
  }))

  default = [
    {
      var1 = "value1"
      var2 = "value2"
    }
  ]
}

output "example" {
  value = jsonencode(var.my_list)
}
