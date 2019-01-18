data "template_file" "example" {
  template = "${var.testmap_test1}-${var.testmap_test2}-${var.testmap_test3}-${var.testmap_testmap2_test}"
}

output "example" {
  value = "${data.template_file.example.rendered}"
}
