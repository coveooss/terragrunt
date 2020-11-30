module github.com/coveooss/terragrunt/v2

go 1.13

require (
	cloud.google.com/go v0.72.0 // indirect
	cloud.google.com/go/storage v1.12.0 // indirect
	github.com/Masterminds/semver/v3 v3.1.1 // indirect
	github.com/Microsoft/go-winio v0.4.15 // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/aws/aws-sdk-go v1.35.33
	github.com/cheekybits/genny v1.0.0
	github.com/cheekybits/is v0.0.0-20150225183255-68e9c0620927 // indirect
	github.com/coveooss/gotemplate/v3 v3.5.4
	github.com/coveooss/multilogger v0.5.2
	github.com/fatih/color v1.10.0
	github.com/go-errors/errors v1.1.1
	github.com/hashicorp/go-getter v1.5.1
	github.com/hashicorp/go-version v1.2.1
	github.com/hashicorp/hcl/v2 v2.7.1
	github.com/hashicorp/terraform v0.13.5
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/kevinburke/ssh_config v0.0.0-20201106050909-4977a11b4351 // indirect
	github.com/matryer/try v0.0.0-20161228173917-9ac251b645a2 // indirect
	github.com/mitchellh/go-testing-interface v1.14.1 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/mitchellh/mapstructure v1.4.0
	github.com/rs/xid v1.2.1
	github.com/sirupsen/logrus v1.7.0
	github.com/stretchr/testify v1.6.1
	github.com/urfave/cli v1.22.5
	github.com/xanzy/ssh-agent v0.3.0 // indirect
	github.com/zclconf/go-cty v1.7.0
	golang.org/x/oauth2 v0.0.0-20201109201403-9fd604954f58 // indirect
	golang.org/x/tools v0.0.0-20201123155928-5bad45943a20 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20201119123407-9b1e624d6bc4 // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/matryer/try.v1 v1.0.0-20150601225556-312d2599e12e
	gopkg.in/yaml.v2 v2.3.0
)

replace github.com/hashicorp/go-getter => github.com/coveord/go-getter v1.5.10

replace github.com/hashicorp/hcl/v2 => github.com/coveord/hcl/v2 v2.7.10
