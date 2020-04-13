module github.com/gruntwork-io/terragrunt

go 1.13

require (
	github.com/aws/aws-sdk-go v1.25.43
	github.com/cheekybits/genny v1.0.0
	github.com/cheekybits/is v0.0.0-20150225183255-68e9c0620927 // indirect
	github.com/coveooss/gotemplate/v3 v3.3.10
	github.com/coveooss/multilogger v0.4.6
	github.com/fatih/color v1.9.0
	github.com/go-errors/errors v1.0.1
	github.com/hashicorp/go-getter v1.4.2-0.20200106182914-9813cbd4eb02
	github.com/hashicorp/go-version v1.2.0
	github.com/hashicorp/hcl/v2 v2.3.0
	github.com/hashicorp/terraform v0.12.23
	github.com/lithammer/dedent v1.1.0
	github.com/matryer/try v0.0.0-20161228173917-9ac251b645a2 // indirect
	github.com/mattn/go-zglob v0.0.1
	github.com/mitchellh/mapstructure v1.1.2
	github.com/rs/xid v1.2.1
	github.com/sergi/go-diff v1.1.0
	github.com/sirupsen/logrus v1.4.2
	github.com/stretchr/testify v1.5.1
	github.com/ulikunitz/xz v0.5.6 // indirect
	github.com/urfave/cli v1.22.2
	github.com/zclconf/go-cty v1.4.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/matryer/try.v1 v1.0.0-20150601225556-312d2599e12e
	gopkg.in/yaml.v2 v2.2.8
)

replace github.com/hashicorp/hcl/v2 => github.com/coveord/hcl/v2 v2.3.1
