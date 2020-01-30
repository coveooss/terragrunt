import_files "test" {
  source   = "bad_source"
  files    = ["*.tf"]
  prefix   = "_test_"
  required = false
}