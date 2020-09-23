
output "direct" {
  value = var.project.project.hello
}

output "indirect" {
  value = var.project.hello
}

output "direct2" {
  value = var.alias.project.project.hello
}

output "indirect2" {
  value = var.alias.project.hello
}
