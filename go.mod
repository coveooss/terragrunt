module github.com/coveooss/terragrunt/v2

go 1.13

require (
	github.com/aws/aws-sdk-go v1.33.1
	github.com/cheekybits/genny v1.0.0
	github.com/cheekybits/is v0.0.0-20150225183255-68e9c0620927 // indirect
	github.com/coveooss/gotemplate/v3 v3.4.2
	github.com/coveooss/multilogger v0.4.8
	github.com/fatih/color v1.9.0
	github.com/go-errors/errors v1.1.1
	github.com/hashicorp/go-getter v1.4.2-0.20200106182914-9813cbd4eb02
	github.com/hashicorp/go-version v1.2.1
	github.com/hashicorp/hcl/v2 v2.6.0
	github.com/hashicorp/terraform v0.12.28
	github.com/lithammer/dedent v1.1.0
	github.com/matryer/try v0.0.0-20161228173917-9ac251b645a2 // indirect
	github.com/mitchellh/mapstructure v1.3.2
	github.com/rs/xid v1.2.1
	github.com/sergi/go-diff v1.1.0
	github.com/sirupsen/logrus v1.6.0
	github.com/stretchr/testify v1.6.1
	github.com/ulikunitz/xz v0.5.7 // indirect
	github.com/urfave/cli v1.22.4
	github.com/zclconf/go-cty v1.5.1
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/matryer/try.v1 v1.0.0-20150601225556-312d2599e12e
	gopkg.in/yaml.v2 v2.3.0
)

replace github.com/hashicorp/hcl/v2 => github.com/coveord/hcl/v2 v2.6.0-coveo-2
