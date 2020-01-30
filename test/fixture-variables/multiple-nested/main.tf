data "template_file" "not-flatten" {
  template = "${var.testmap["test1"]}${var.main_testmap["test2"]}${var.local_testmap["test3"]}"
}

data "template_file" "flatten" {
  template = "${var.testmap_test1}${var.main_testmap_test2}${var.local_testmap_test3}"
}

output "not-flatten" {
  value = "${data.template_file.not-flatten.rendered}"
}

output "flatten" {
  value = "${data.template_file.flatten.rendered}"
}
