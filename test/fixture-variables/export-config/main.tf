data "template_file" "example" {
  template = var.ImportVariables[0].Vars[0]
}

output "example" {
  value = data.template_file.example.rendered
}
