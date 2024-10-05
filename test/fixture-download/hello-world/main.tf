variable "name" {
  description = "Specify a name"
}

output "test" {
  value = "${module.hello.hello}, ${var.name}"
}

module "hello" {
  source = "./hello"
}
