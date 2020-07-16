variable "ignored_razor" {
  description = "This variable should not be loaded by default values since it contains Razor code"
  default     = "@(2+3)"
}

variable "ignored_gotemplate" {
  description = "This variable should not be loaded by default values since it contains gotemplate code"
  default     = "{{ mul 4 2 }}"
}
