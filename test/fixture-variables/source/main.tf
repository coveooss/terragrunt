data "template_file" "example" {
  template = "${var.hello1}${var.hello2}"
}

output "example" {
  value = data.template_file.example.rendered
}
