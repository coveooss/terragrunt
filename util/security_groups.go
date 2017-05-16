package util

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// GetSecurityGroupTags returns a map of the tags associated with a security group
func GetSecurityGroupTags(groupName string) (result map[string]string, err error) {
	result = map[string]string{}

	s := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := ec2.New(s)
	sgGroups, err := svc.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		GroupNames: []*string{&groupName},
	})

	if err != nil || sgGroups.SecurityGroups[0].Tags == nil {
		return
	}

	for _, tag := range sgGroups.SecurityGroups[0].Tags {
		result[*tag.Key] = *tag.Value
	}
	return
}
