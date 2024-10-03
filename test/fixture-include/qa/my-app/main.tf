terraform {
  backend "s3" {}
}

output "text" {
  value = "Hello, I am an output"
}
