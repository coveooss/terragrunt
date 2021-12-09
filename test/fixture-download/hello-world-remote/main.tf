data "template_file" "test" {
  template = "Hello ${var.name}, how are you?"
}

variable "name" {
  description = "Specify a name"
}

output "test" {
  value = data.template_file.test.rendered
}
