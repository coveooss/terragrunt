data "template_file" "test" {
  template = "hello remote from ${module.hello.hello}\n${module.remote.test}"
}

variable "name" {
  description = "Specify a name"
  default     = "local"
}

output "test" {
  value = data.template_file.test.rendered
}

module "hello" {
  source = "./hello"
  name   = var.name
}

module "remote" {
  source = "github.com/coveooss/terragrunt.git//test/fixture-download/hello-world?ref=download_test"
  name   = var.name
}
