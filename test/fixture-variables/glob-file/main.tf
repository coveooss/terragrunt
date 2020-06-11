data "template_file" "example" {
  template = "${var.var1}-${var.var2}-${var.var3}-${var.var4}"
}

output "example" {
  value = data.template_file.example.rendered
}
