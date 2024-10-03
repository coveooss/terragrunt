output "example" {
  value = "${var.var1}-${var.var2}-${var.loaded.var1}-${var.loaded.var2}"
}
