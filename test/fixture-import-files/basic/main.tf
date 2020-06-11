data "template_file" "example" {
  template = var.var
}

output "example" {
  value = data.template_file.example.rendered
}
