package dns

//go:generate mockgen -package dns -source=./route53api.go -destination=./route53api_mock.go

import (
	"github.com/aws/aws-sdk-go/service/route53"
)

type Route53API interface {
	ListResourceRecordSets(*route53.ListResourceRecordSetsInput) (*route53.ListResourceRecordSetsOutput, error)
	ChangeResourceRecordSets(*route53.ChangeResourceRecordSetsInput) (*route53.ChangeResourceRecordSetsOutput, error)
}
