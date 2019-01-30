terragrunt = {
  import_files "test" {
    source = "source"
    files  = ["*.tf"]
  }
}
