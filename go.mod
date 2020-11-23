module github.com/coveooss/terragrunt/v2

go 1.13

require (
	github.com/aws/aws-sdk-go v1.35.20
	github.com/cheekybits/genny v1.0.0
	github.com/cheekybits/is v0.0.0-20150225183255-68e9c0620927 // indirect
	github.com/coveooss/gotemplate/v3 v3.5.2
	github.com/coveooss/multilogger v0.5.2
	github.com/fatih/color v1.10.0
	github.com/go-errors/errors v1.1.1
	github.com/hashicorp/go-getter v1.5.0
	github.com/hashicorp/go-version v1.2.1
	github.com/hashicorp/hcl/v2 v2.7.1
	github.com/hashicorp/terraform v0.13.5
	github.com/lithammer/dedent v1.1.0
	github.com/matryer/try v0.0.0-20161228173917-9ac251b645a2 // indirect
	github.com/mitchellh/mapstructure v1.3.3
	github.com/rs/xid v1.2.1
	github.com/sergi/go-diff v1.1.0
	github.com/sirupsen/logrus v1.7.0
	github.com/stretchr/testify v1.6.1
	github.com/urfave/cli v1.22.5
	github.com/zclconf/go-cty v1.7.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/matryer/try.v1 v1.0.0-20150601225556-312d2599e12e
	gopkg.in/yaml.v2 v2.3.0
)

replace github.com/hashicorp/go-getter => github.com/coveord/go-getter v1.5.10

replace github.com/hashicorp/hcl/v2 => github.com/coveord/hcl/v2 v2.7.10
