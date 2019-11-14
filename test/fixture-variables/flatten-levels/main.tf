data "template_file" "example" {
  template = "${var.infra_common_testmap["test1"]}-${var.infra_common_testmap["test2"]}-${var.infra_common_test3}-${var.infra_other_hello}"
}

output "example" {
  value = "${data.template_file.example.rendered}"
}

output "example_gotemplate" {
  value = "@(infra.common.testmap.test1)-@(infra.common.testmap.test2)-@(infra.common.test3)-@(infra.other.hello)"
}
