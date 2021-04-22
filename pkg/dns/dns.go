package dns

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

const (
	HostnameAnnotationKey = "external-dns.alpha.kubernetes.io/hostname"
	// The annotation used for defining the desired ingress target
	targetAnnotationKey = "external-dns.alpha.kubernetes.io/target"
	// The annotation used for defining the desired DNS record TTL
	ttlAnnotationKey = "external-dns.alpha.kubernetes.io/ttl"
	// The annotation used for switching to the alias record types e. g. AWS Alias records instead of a normal CNAME
	aliasAnnotationKey = "external-dns.alpha.kubernetes.io/alias"
	// external-dns defined annotation keys for route53
	HealthCheckIdAnnotationKey = "external-dns.alpha.kubernetes.io/aws-health-check-id"
	weightAnnotationKey        = "external-dns.alpha.kubernetes.io/aws-weight"
	setIdentifierAnnotationKey = "external-dns.alpha.kubernetes.io/set-identifier"
	// external-route53 defined annotation keys
	// specified record-type: ex: A, CNAME
	recordTypeAnnotationKey = "external-route53.io/record-type"
	// set if health check will be created
	HealthCheckAnnotationKey = "external-route53.io/health-check"
	// specifiy zone id
	zoneAnnotationKey = "external-route53.io/hosted-zone-id"
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

func Ensure(svc *corev1.Service) error {
	ro, err := toUpsertRecordSetOpt(svc)
	if err != nil {
		return err
	}
	return ensureRecord(ro)
}

func toUpsertRecordSetOpt(svc *corev1.Service) (UpsertRecordSetOpt, error) {
	var w, ttl int = 1, 10
	_, ok := svc.Annotations[weightAnnotationKey]
	if ok {
		ret, err := strconv.Atoi(svc.Annotations[weightAnnotationKey])
		if err != nil {
			return UpsertRecordSetOpt{}, err
		}
		w = ret
	}
	_, ok = svc.Annotations[ttlAnnotationKey]
	if ok {
		ret, err := strconv.Atoi(svc.Annotations[ttlAnnotationKey])
		if err != nil {
			return UpsertRecordSetOpt{}, err
		}
		ttl = ret
	}
	var alias bool
	_, ok = svc.Annotations[aliasAnnotationKey]
	if ok {
		ret, err := strconv.ParseBool(svc.Annotations[aliasAnnotationKey])
		if err != nil {
			return UpsertRecordSetOpt{}, err
		}
		alias = ret
	} else {
		alias = svc.Spec.Type == corev1.ServiceTypeExternalName
	}
	recordType, ok := svc.Annotations[recordTypeAnnotationKey]
	if !ok {
		recordType = "A"
	}
	identifier, ok := svc.Annotations[setIdentifierAnnotationKey]
	if !ok {
		identifier = fmt.Sprintf("%s/%s/%s", svc.Namespace, svc.Name, svc.UID)
	}
	var thn, tip string = "", ""
	switch svc.Spec.Type {
	case corev1.ServiceTypeExternalName:
		thn = svc.Spec.ExternalName
	case corev1.ServiceTypeLoadBalancer:
		tip = svc.Status.LoadBalancer.Ingress[0].IP
	}
	ro := UpsertRecordSetOpt{
		Hostname:        svc.Annotations[HostnameAnnotationKey],
		Type:            recordType,
		Identifier:      identifier,
		HealthCheckID:   svc.Annotations[HealthCheckIdAnnotationKey],
		HostedZoneID:    svc.Annotations[zoneAnnotationKey],
		Weight:          w,
		TTL:             ttl,
		Alias:           alias,
		TargetHostname:  thn,
		TargetIPAddress: tip,
	}
	if err := validateRecordSetOpt(ro); err != nil {
		return UpsertRecordSetOpt{}, err
	}
	return ro, nil
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
	var ttl *int64
	var at *route53.AliasTarget = nil
	var rrs []*route53.ResourceRecord = nil
	if ro.Alias {
		at = &route53.AliasTarget{
			EvaluateTargetHealth: aws.Bool(true),
			HostedZoneId:         aws.String(ro.HostedZoneID),
			DNSName:              aws.String(ro.TargetHostname),
		}
		ttl = nil
	} else {
		rrs = []*route53.ResourceRecord{
			{Value: aws.String(ro.TargetIPAddress)},
		}
		ttl = aws.Int64(int64(ro.TTL))
	}
	changes := []*route53.Change{
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
				TTL:             ttl,
			},
		},
	}
	logrus.Info(changes)
	_, err := r.ChangeResourceRecordSets(&route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(ro.HostedZoneID),
		ChangeBatch: &route53.ChangeBatch{
			Comment: aws.String("change from external-route53"),
			Changes: changes,
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
	if ro.Alias && ro.TargetHostname == "" {
		return errors.New("Alias record enabled but target hostname is not defined")
	}
	if !ro.Alias && ro.TargetIPAddress == "" {
		return errors.New("Alias record disabled but target IP Address is not defined")
	}
	return nil
}

func supportedType(t string) bool {
	// only A record is supported
	return t == "A"
}
