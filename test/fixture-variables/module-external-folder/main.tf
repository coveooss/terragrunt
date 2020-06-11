module "my_module" {
  source      = "@pwd()/fixture-variables/external_module"
  my_variable = var.my_variable
}

output "example" {
  value = module.my_module.example
}
