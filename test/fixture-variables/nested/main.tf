data "template_file" "example" {
  template = "${var.nested1_var}-${var.nested2_var}"
}

output "example" {
  value = "${data.template_file.example.rendered}"
}
