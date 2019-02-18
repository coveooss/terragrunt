data "template_file" "example" {
  template = "${var.bool ? var.list[1] : var.list[0]}-${var.int > 2 ? var.string : ""}"
}

output "example" {
  value = "${data.template_file.example.rendered}"
}
