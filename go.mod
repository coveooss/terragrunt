module github.com/coveooss/terragrunt/v2

go 1.13

require (
	github.com/aws/aws-sdk-go v1.37.0
	github.com/cheekybits/genny v1.0.0
	github.com/cheekybits/is v0.0.0-20150225183255-68e9c0620927 // indirect
	github.com/coveooss/gotemplate/v3 v3.6.1
	github.com/coveooss/multilogger v0.5.2
	github.com/fatih/color v1.10.0
	github.com/go-errors/errors v1.1.1
	github.com/hashicorp/go-getter v1.5.1
	github.com/hashicorp/go-version v1.2.1
	github.com/hashicorp/hcl/v2 v2.8.2
	github.com/hashicorp/terraform v0.14.5
	github.com/matryer/try v0.0.0-20161228173917-9ac251b645a2 // indirect
	github.com/mitchellh/mapstructure v1.4.1
	github.com/rs/xid v1.2.1
	github.com/sirupsen/logrus v1.7.0
	github.com/stretchr/testify v1.7.0
	github.com/urfave/cli v1.22.5
	github.com/zclconf/go-cty v1.7.1
	golang.org/x/tools v0.1.0 // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/matryer/try.v1 v1.0.0-20150601225556-312d2599e12e
	gopkg.in/yaml.v2 v2.4.0
)

replace (
	github.com/hashicorp/go-getter => github.com/coveord/go-getter v1.5.12
	// sum DB is messed up for v2.8.12
	github.com/hashicorp/hcl/v2 => github.com/coveord/hcl/v2 v2.8.102
)
