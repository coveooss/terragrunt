terraform {
  backend "s3" {}
}

output "test" {
  value = "Hello, I am a output"
}
