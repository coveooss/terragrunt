data "template_file" "nested" {
  template = "${var.testmap.test1}${var.main.testmap.test2}${var.local.testmap.test3}"
}

output "nested" {
  value = "${data.template_file.nested.rendered}"
}

