terraform {
  source = "github.com/coveooss/terragrunt.git//test/fixture-download/hello-world?ref=v0.9.9"
}
inputs = {
  name = "World"
}
