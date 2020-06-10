data "template_file" "example" {
  template = var.hello
}

output "example" {
  value = data.template_file.example.rendered
}
