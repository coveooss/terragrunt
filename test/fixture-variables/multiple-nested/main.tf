output "nested" {
  value = "${var.testmap.test1}${var.main.testmap.test2}${var.local.testmap.test3}"
}
