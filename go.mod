module github.com/coveooss/terragrunt/v2

go 1.23

require (
	github.com/alecthomas/kingpin/v2 v2.4.0
	github.com/aws/aws-sdk-go-v2 v1.32.1
	github.com/aws/aws-sdk-go-v2/config v1.27.42
	github.com/aws/aws-sdk-go-v2/credentials v1.17.40
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.36.1
	github.com/aws/aws-sdk-go-v2/service/s3 v1.65.1
	github.com/aws/aws-sdk-go-v2/service/ssm v1.55.1
	github.com/aws/aws-sdk-go-v2/service/sts v1.32.1
	github.com/cheekybits/genny v1.0.0
	github.com/coveooss/gotemplate/v3 v3.9.1
	github.com/coveooss/multilogger v0.6.0
	github.com/fatih/color v1.17.0
	github.com/go-errors/errors v1.5.1
	github.com/hashicorp/go-getter/v2 v2.2.3
	github.com/hashicorp/go-version v1.7.0
	github.com/hashicorp/hcl/v2 v2.22.0
	github.com/hashicorp/terraform v0.15.3 // Terraform must be pinned to a specific version to avoid breaking changes
	github.com/hashicorp/terraform-config-inspect v0.0.0-20240801114854-6714b46f5fe4
	github.com/mitchellh/mapstructure v1.5.0
	github.com/rs/xid v1.6.0
	github.com/sirupsen/logrus v1.9.3
	github.com/stretchr/testify v1.9.0
	github.com/urfave/cli v1.22.15
	github.com/zclconf/go-cty v1.15.0
	gopkg.in/matryer/try.v1 v1.0.0-20150601225556-312d2599e12e
	gopkg.in/yaml.v3 v3.0.1
)

require (
	dario.cat/mergo v1.0.1 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.3.0 // indirect
	github.com/Masterminds/sprig/v3 v3.3.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/PuerkitoBio/goquery v1.10.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/alecthomas/units v0.0.0-20240927000941-0f3dac36c52b // indirect
	github.com/andybalholm/cascadia v1.3.2 // indirect
	github.com/apparentlymart/go-cidr v1.1.0 // indirect
	github.com/apparentlymart/go-textseg/v12 v12.0.0 // indirect
	github.com/apparentlymart/go-textseg/v15 v15.0.0 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.6.6 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.16 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.20 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.20 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.3.20 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.4.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.10.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.18.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.24.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.28.1 // indirect
	github.com/aws/smithy-go v1.22.0 // indirect
	github.com/bgentry/go-netrc v0.0.0-20140422174119-9fd32a8b3d3d // indirect
	github.com/bmatcuk/doublestar v1.3.4 // indirect
	github.com/cheekybits/is v0.0.0-20150225183255-68e9c0620927 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.5 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/drhodes/goLorem v0.0.0-20220328165741-da82e5b29246 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-safetemp v1.0.0 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hashicorp/terraform-svchost v0.1.1 // indirect
	github.com/huandu/xstrings v1.5.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/klauspost/compress v1.17.10 // indirect
	github.com/matryer/try v0.0.0-20161228173917-9ac251b645a2 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-testing-interface v1.14.1 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sergi/go-diff v1.3.1 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/spf13/cast v1.7.0 // indirect
	github.com/src-d/gcfg v1.4.0 // indirect
	github.com/ulikunitz/xz v0.5.12 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	github.com/xhit/go-str2duration/v2 v2.1.0 // indirect
	github.com/zclconf/go-cty-yaml v1.1.0 // indirect
	golang.org/x/crypto v0.28.0 // indirect
	golang.org/x/net v0.30.0 // indirect
	golang.org/x/sys v0.26.0 // indirect
	golang.org/x/term v0.25.0 // indirect
	golang.org/x/text v0.19.0 // indirect
	gopkg.in/src-d/go-billy.v4 v4.3.2 // indirect
	gopkg.in/src-d/go-git.v4 v4.13.1 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
)

replace (
	github.com/hashicorp/go-getter => github.com/coveord/go-getter v1.7.105
	github.com/hashicorp/hcl/v2 => github.com/coveord/hcl/v2 v2.8.102
)
