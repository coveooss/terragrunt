terraform {
  backend "s3" {}
}

output "example" {
  value = "hello, world"
}
