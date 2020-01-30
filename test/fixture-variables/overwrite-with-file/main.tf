
output "example" {
  value = "${var.not_overwritten} -> ${var.value_set_in_file}"
}
