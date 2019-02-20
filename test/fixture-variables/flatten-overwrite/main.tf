data "template_file" "example" {
  template = "${var.testmap["test1"]}-${var.testmap["test2"]}-${var.testmap["test3"]}"
}

output "example" {
  value = "${data.template_file.example.rendered}"
}
