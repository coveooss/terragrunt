data "template_file" "example" {
  template = "${var.nested1["var"]}-${var.nested2["var"]}"
}

output "example" {
  value = "${data.template_file.example.rendered}"
}
