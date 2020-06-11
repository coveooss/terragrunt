module "my_module" {
  source = "./my-module"
}

output "example" {
  value = module.my_module.example
}
