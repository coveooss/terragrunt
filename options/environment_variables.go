package options

// All environment variables that could be used to configure Terragrunt
const (
	EnvCacheFolder         = "TERRAGRUNT_CACHE_FOLDER"          // Used to configure the cache folder (optional, default determined by os temp folder)
	EnvConfig              = "TERRAGRUNT_CONFIG"                // Used to configure the location of the Terragrunt configuration file (optional, default terragrunt.hcl in the current folder)
	EnvDebug               = "TERRAGRUNT_DEBUG"                 // Used to enable Terragrunt debug mode
	EnvFlushDelay          = "TERRAGRUNT_FLUSH_DELAY"           // Used to configure the flush delay on long -all operation (default 60s)
	EnvLoggingLevel        = "TERRAGRUNT_LOGGING_LEVEL"         // Used to configure the current logging level
	EnvLoggingFileDir      = "TERRAGRUNT_LOGGING_FILE_DIR"      // Used to configure the directory where (verbose) file logs will be saved
	EnvLoggingFileLevel    = "TERRAGRUNT_LOGGING_FILE_LEVEL"    // Used to configure the logging level in files
	EnvSource              = "TERRAGRUNT_SOURCE"                // Used to configure the location of the Terraform source folder (optional, default determined by source in the terragrunt.terraform object
	EnvSourceUpdate        = "TERRAGRUNT_SOURCE_UPDATE"         // Used to configure the --terragrunt-source-update option (flushes the cache) (optional)
	EnvTFPath              = "TERRAGRUNT_TFPATH"                // Used to configure the path to the terraform command (optional, default terraform)
	EnvWorkers             = "TERRAGRUNT_WORKERS"               // Used to configure the maximum number of concurrent workers (optional)
	EnvApplyTemplate       = "TERRAGRUNT_TEMPLATE"              // Used to configure whether or not go template should be applied on terraform (.tf and .tfvars) file
	EnvTemplatePatterns    = "TERRAGRUNT_TEMPLATE_PATTERNS"     // Used to configure the extra files (other than .tf) that should be processed by go template
	EnvBootConfigs         = "TERRAGRUNT_BOOT_CONFIGS"          // Used to set defaults configuration when launching terragrunt
	EnvPreBootConfigs      = "TERRAGRUNT_PREBOOT_CONFIGS"       // Used to set defaults configuration when launching terragrunt (loaded before user files)
	EnvIncludeEmptyFolders = "TERRAGRUNT_INCLUDE_EMPTY_FOLDERS" // Used to set the option terragrunt-include-empty-folders
)

// All environment variables that are published during Terragrunt execution to share current context during shell execution
const (
	EnvArgs            = "TERRAGRUNT_ARGS"             // Used to publish the supplied arguments to the Terragrunt command
	EnvCommand         = "TERRAGRUNT_COMMAND"          // Used to publish the current Terragrunt command
	EnvExtraCommand    = "TERRAGRUNT_EXTRA_COMMAND"    // Used to publish the name of the actual running command
	EnvLaunchFolder    = "TERRAGRUNT_LAUNCH_FOLDER"    // Used to publish the launch folder from where the Terragrunt operation has been launched
	EnvRunID           = "TERRAGRUNT_RUN_ID"           // Used to publish the current run id, this is unique to each Terragrunt execution, but can be used to link -all operations
	EnvSourceFolder    = "TERRAGRUNT_SOURCE_FOLDER"    // Used to publish the current Terraform source folder used
	EnvTemporaryFolder = "TERRAGRUNT_TEMPORARY_FOLDER" // Used to publish the Terragrunt temporary folder used
	EnvTFVersion       = "TERRAFORM_VERSION"           // Used to publish the Terraform version
	EnvVersion         = "TERRAGRUNT_VERSION"          // Used to publish the Terragrunt version
)

// All environment variables that are published during Terragrunt execution to indicate the current execution status
const (
	EnvLastError  = "TERRAGRUNT_LAST_ERROR"  // Used to publish the last executed command error message
	EnvLastStatus = "TERRAGRUNT_LAST_STATUS" // Used to publish the last executed command exit code
	EnvError      = "TERRAGRUNT_ERROR"       // Used to publish the cumulated command error if there are
	EnvStatus     = "TERRAGRUNT_STATUS"      // Used to publish the status of the execution flow
)
