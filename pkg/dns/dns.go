package dns

import (
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	corev1 "k8s.io/api/core/v1"
)

const (
	hostnameAnnotationKey = "external-dns.alpha.kubernetes.io/hostname"
	// The annotation used for defining the desired ingress target
	targetAnnotationKey = "external-dns.alpha.kubernetes.io/target"
	// The annotation used for defining the desired DNS record TTL
	ttlAnnotationKey = "external-dns.alpha.kubernetes.io/ttl"
	// The annotation used for switching to the alias record types e. g. AWS Alias records instead of a normal CNAME
	aliasAnnotationKey = "external-dns.alpha.kubernetes.io/alias"
	// external-dns defined annotation keys for route53
	healhCheckIdAnnotationKey  = "external-dns.alpha.kubernetes.io/aws-health-check-id"
	weightAnnotationKey        = "external-dns.alpha.kubernetes.io/aws-weight"
	setIdentifierAnnotationKey = "external-dns.alpha.kubernetes.io/set-identifier"
	// external-route53 defined annotation keys
	// specified record-type: ex: A, CNAME
	recordTypeAnnotationKey = "external-route53.io/record-type"
	// set if health check will be created
	healthCheckAnnotationKey = "external-route53.io/health-check"
	// specifiy zone id
	zoneAnnotationKey = "external-route53.io/zone"
)

type UpsertRecordSetOpt struct {
	Hostname        string
	Type            string
	Identifier      string
	HealthCheckID   string
	HostedZoneID    string
	Weight          int
	TTL             int
	Alias           bool
	TargetHostname  string
	TargetIPAddress string
}

func SatisfiedAliasRecordCreation(svc *corev1.Service) error {
	return nil
}

func ensureRecord(ro UpsertRecordSetOpt) error {
	if err := validateRecordSetOpt(ro); err != nil {
		return err
	}
	return upsert(ro)
}

func recordExists(ro UpsertRecordSetOpt) (bool, error) {
	mySession := session.Must(session.NewSession())
	r := route53.New(mySession)
	out, err := r.ListResourceRecordSets(&route53.ListResourceRecordSetsInput{
		HostedZoneId:          aws.String(ro.HostedZoneID),
		StartRecordIdentifier: &ro.Identifier,
		StartRecordName:       &ro.Hostname,
		StartRecordType:       &ro.Type,
	})
	if err != nil {
		return false, err
	}

	if len(out.ResourceRecordSets) < 1 {
		return false, errors.New("entry was not found")
	}
	for _, rr := range out.ResourceRecordSets {
		if rr.SetIdentifier != nil && strings.Contains(*rr.Name, ro.Hostname) && *rr.SetIdentifier == ro.Identifier {
			return true, nil
		}
	}
	return false, nil
}

func upsert(ro UpsertRecordSetOpt) error {
	return query("UPSERT", ro)
}

func delete(ro UpsertRecordSetOpt) error {
	err := query("DELETE", ro)
	if strings.Contains(err.Error(), "but it was not found") {
		return nil
	}
	return err

}

func query(action string, ro UpsertRecordSetOpt) error {
	var healthCheckId *string = nil
	if ro.HealthCheckID != "" {
		healthCheckId = &ro.HealthCheckID
	}
	mySession := session.Must(session.NewSession())
	r := route53.New(mySession)
	var at *route53.AliasTarget = nil
	var rrs []*route53.ResourceRecord = nil
	if ro.Alias {
		at = &route53.AliasTarget{
			EvaluateTargetHealth: aws.Bool(true),
			HostedZoneId:         aws.String(ro.HostedZoneID),
			DNSName:              aws.String(ro.TargetHostname),
		}
	} else {
		rrs = []*route53.ResourceRecord{
			{Value: aws.String(ro.TargetIPAddress)},
		}
	}
	_, err := r.ChangeResourceRecordSets(&route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(ro.HostedZoneID),
		ChangeBatch: &route53.ChangeBatch{
			Comment: aws.String("change from external-route53"),
			Changes: []*route53.Change{
				{
					Action: aws.String(action),
					ResourceRecordSet: &route53.ResourceRecordSet{
						Name:            aws.String(ro.Hostname),
						AliasTarget:     at,
						ResourceRecords: rrs,
						SetIdentifier:   aws.String(ro.Identifier),
						Weight:          aws.Int64(int64(ro.Weight)),
						HealthCheckId:   healthCheckId,
						Type:            aws.String(ro.Type),
						TTL:             aws.Int64(int64(ro.TTL)),
					},
				},
			},
		},
	})
	if err != nil {
		return err
	}
	return nil
}

func validateRecordSetOpt(ro UpsertRecordSetOpt) error {
	if ro.HostedZoneID == "" {
		return errors.New("hosted zone id is not found")
	}
	if ro.Hostname == "" {
		return errors.New("hostname is not found")
	}
	if ro.Identifier == "" {
		return errors.New("identifier is not found")
	}
	if !supportedType(ro.Type) {
		return errors.New("this type is not supported")
	}
	if ro.TTL < 10 {
		return errors.New("TTL must be over 10s")
	}
	return nil
}

func supportedType(t string) bool {
	// only A record is supported
	return t == "A"
}
