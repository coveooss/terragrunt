output "example" {
  value = "${var.bool ? var.list[1] : var.list[0]}-${var.int > 2 ? var.string : ""}"
}
