data "template_file" "example" {
  template = "${var.testmap["test1"]}-${var.testmap["test2"]}-@(not_flattened.testmap2.test1)-@(not_flattened.testmap2.test2)"
}

output "example" {
  value = "${data.template_file.example.rendered}"
}
