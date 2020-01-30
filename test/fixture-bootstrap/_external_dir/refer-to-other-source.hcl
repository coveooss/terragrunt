terraform {
  source = "${get_parent_tfvars_dir()}/${var.source_path}/${path_relative_to_include()}"
}
