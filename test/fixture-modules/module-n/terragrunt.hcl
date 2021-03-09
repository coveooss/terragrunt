terraform {
  source = "test"
}

dependencies {
  paths = ["../module-m", "./module-n-child"]
}
