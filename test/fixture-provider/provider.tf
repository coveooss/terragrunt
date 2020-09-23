provider "template" {
  alias = "test"
}

# Create an arbitrary local resource
data "template_file" "test" {
  provider = "template.@(`test`)"
  template = "Everything is fine"
}

output "ok" {
  value = data.template_file.test.rendered
}
