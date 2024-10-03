output "example" {
  value = "${var.testmap.test1}-${var.testmap.test2}-${var.nested.testmap2.test1}-${var.nested.testmap2.test2}-@(nested.testmap2.test1)-@(nested.testmap2.test2)"
}
