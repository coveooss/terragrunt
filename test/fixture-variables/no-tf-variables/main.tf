data "template_file" "example" {
  template = "@(hello1)@(hello2)@(hello3)"
}

output "example" {
  value = data.template_file.example.rendered
}
