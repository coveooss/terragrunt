data "template_file" "example" {
  template = "${var.var1}-${var.var2}-${var.loaded.var1}-${var.loaded.var2}"
}

output "example" {
  value = "${data.template_file.example.rendered}"
}
