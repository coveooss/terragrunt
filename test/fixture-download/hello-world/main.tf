variable "name" {
  description = "Specify a name"
}

output "test" {
  value = "${module.hello.hello}, ${var.name}"
}

module "hello" {
  source = "./hello"
}

module "remote" {
  source = "github.com/coveooss/terragrunt.git//test/fixture-download/hello-world?ref=download_test"
  name   = var.name
}
