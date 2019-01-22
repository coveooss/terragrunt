data "template_file" "example" {
  template = "${var.var1}-${var.var2}-${var.loaded_var1}-${var.loaded_var2}"
}

output "example" {
  value = "${data.template_file.example.rendered}"
}
