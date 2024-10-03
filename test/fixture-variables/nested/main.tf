output "example" {
  value = "${var.nested1["var"]}-${var.nested2["var"]}"
}
