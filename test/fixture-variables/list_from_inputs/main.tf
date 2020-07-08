variable "my_list" {

  type = list(object({
    var1 = string
    var2 = string
  }))
}

output "example" {
  value = jsonencode(var.my_list)
}
