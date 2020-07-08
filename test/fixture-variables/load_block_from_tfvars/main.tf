variable "my_object" {}

output "example" {
  value = var.my_object.nested.double_nested
}
