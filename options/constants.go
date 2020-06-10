package options

// Global constants
const (
	IgnoreFile               = "terragrunt.ignore"
	IgnoreFileNonInteractive = "terragrunt-non-interactive.ignore"
	DefaultConfigName        = "terragrunt.hcl"
)

var (
	configNames []string

	// TerraformFilesPatterns lists the patterns of expected files considered as terraform content.
	TerraformFilesPatterns = []string{"*.tf", "*.tf.json"}

	// TerraformFilesPatternsExtended lists the patterns of expected files considered as terraform content (including other file format).
	TerraformFilesPatternsExtended = append(TerraformFilesPatterns, "*.tf.yaml")

	// TerraformFilesTemplates list all files that may contains terraform code (while expanded).
	TerraformFilesTemplates []string
)

func init() {
	// We left the user many choices to name its terragrunt config file
	extensions := []string{".hcl", ".json", ".hcl.json", ".yaml", ".yml", ".config", ""}
	configNames = make([]string, 0, 2*len(extensions))
	for _, ext := range extensions {
		configNames = append(configNames, ".terragrunt"+ext)
		configNames = append(configNames, "terragrunt"+ext)
	}

	// We add support for template files in the list of terraform files
	TerraformFilesTemplates = make([]string, 0, len(TerraformFilesPatternsExtended)*3)
	for _, file := range TerraformFilesPatternsExtended {
		for _, ext := range []string{"", ".gt", ".template"} {
			TerraformFilesTemplates = append(TerraformFilesTemplates, file+ext)
		}
	}
}
