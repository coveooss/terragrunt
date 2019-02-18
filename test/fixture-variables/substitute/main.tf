data "template_file" "example" {
  template = "${var.var1}-${var.var2}"
}

output "example" {
  value = "${data.template_file.example.rendered}"
}
