data "template_file" "example" {
  template = "${var.infra_common_testmap_test1}-${var.infra_common_testmap_test2}-${var.infra_common_test3}-${var.infra_other_hello}"
}

output "example" {
  value = "${data.template_file.example.rendered}"
}
