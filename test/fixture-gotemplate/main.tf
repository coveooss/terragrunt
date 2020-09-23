data "template_file" "@String(`This is an example`).Fields().Join(`_`)" {
  template = "ok"
}

output "result" {
  value = data.template_file.This_is_an_example.rendered
}

variable "test1" {
  default = "I am test 1"
}

variable "test2" {
  default = "I am test 2"
}

# Using gotemplate to render values that are defined as a default value for var
output "test1" {
  value = "@test1"
}

output "test2" {
  value = "@test2"
}

output "json1" {
  value = "@json1"
}

output "json2" {
  value = "@json2"
}
