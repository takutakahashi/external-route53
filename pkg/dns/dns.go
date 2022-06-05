package dns

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elbv2"
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
	ElbHostedZoneID string
	Weight          int
	TTL             int
	Alias           bool
	TargetHostname  string
	TargetIPAddress string
	TXTPrefix       string
}

type Dns struct {
	client Route53API
}

func NewDns() Dns {
	mySession := session.Must(session.NewSession())
	d := Dns{
		client: route53.New(mySession),
	}
	return d
}

func SatisfiedAliasRecordCreation(svc *corev1.Service) error {
	return nil
}

func (d *Dns) Ensure(svc *corev1.Service) error {
	ro, err := d.toUpsertRecordSetOpt(svc)
	if err != nil {
		return err
	}
	return d.ensureRecord(ro)
}
func (d *Dns) Delete(svc *corev1.Service) error {
	ro, err := d.toUpsertRecordSetOpt(svc)
	if err != nil {
		return err
	}
	return d.delete(ro)
}

func (d *Dns) toUpsertRecordSetOpt(svc *corev1.Service) (UpsertRecordSetOpt, error) {
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
		switch {
		case svc.Spec.Type == corev1.ServiceTypeExternalName:
			alias = true
		case svc.Spec.Type == corev1.ServiceTypeLoadBalancer:
			alias = svc.Status.LoadBalancer.Ingress[0].IP == ""
		}
	}
	recordType, ok := svc.Annotations[recordTypeAnnotationKey]
	if !ok {
		recordType = "A"
	}
	identifier, ok := svc.Annotations[setIdentifierAnnotationKey]
	if !ok {
		identifier = fmt.Sprintf("%s/%s/%s", svc.Namespace, svc.Name, svc.UID)
	}

	hostedZoneID := os.Getenv("HOSTED_ZONE_ID")
	if s, ok := svc.Annotations[zoneAnnotationKey]; ok {
		hostedZoneID = s
	}

	// ELB
	elbHostedZoneID := ""
	ingress := svc.Status.LoadBalancer.Ingress
	r := regexp.MustCompile(`\.elb\.[A-Za-z0-9\-]+\.amazonaws\.com$`)
	if len(ingress) > 0 && r.MatchString(ingress[0].Hostname) {
		zoneID, err := getElbCanonicalHostedZoneId(ingress[0].Hostname)
		if err != nil {
			return UpsertRecordSetOpt{}, err
		}
		elbHostedZoneID = *zoneID
	}

	var thn, tip string = "", ""
	switch svc.Spec.Type {
	case corev1.ServiceTypeExternalName:
		thn = svc.Spec.ExternalName
	case corev1.ServiceTypeLoadBalancer:
		tip = svc.Status.LoadBalancer.Ingress[0].IP
		if tip == "" {
			thn = svc.Status.LoadBalancer.Ingress[0].Hostname
		}
	}
	ro := UpsertRecordSetOpt{
		Hostname:        svc.Annotations[HostnameAnnotationKey],
		Type:            recordType,
		Identifier:      identifier,
		HealthCheckID:   svc.Annotations[HealthCheckIdAnnotationKey],
		HostedZoneID:    hostedZoneID,
		ElbHostedZoneID: elbHostedZoneID,
		Weight:          w,
		TTL:             ttl,
		Alias:           alias,
		TargetHostname:  thn,
		TargetIPAddress: tip,
		TXTPrefix:       "extr53-",
	}
	if err := d.validateRecordSetOpt(ro); err != nil {
		return UpsertRecordSetOpt{}, err
	}
	return ro, nil
}

func (d *Dns) ensureRecord(ro UpsertRecordSetOpt) error {
	if err := d.validateRecordSetOpt(ro); err != nil {
		return err
	}
	return d.upsert(ro)
}

func (d Dns) recordExists(ro UpsertRecordSetOpt) (bool, error) {
	out, err := d.client.ListResourceRecordSets(&route53.ListResourceRecordSetsInput{
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
func (d *Dns) upsert(ro UpsertRecordSetOpt) error {
	return d.query("UPSERT", ro)
}

func (d *Dns) delete(ro UpsertRecordSetOpt) error {
	err := d.query("DELETE", ro)
	if err != nil && strings.Contains(err.Error(), "but it was not found") {
		return nil
	}
	return err

}

func (d *Dns) query(action string, ro UpsertRecordSetOpt) error {
	var healthCheckId *string = nil
	if ro.HealthCheckID != "" {
		healthCheckId = &ro.HealthCheckID
	}
	var ttl *int64
	var at *route53.AliasTarget = nil
	var rrs []*route53.ResourceRecord = nil
	if ro.Alias {
		zoneId := ro.HostedZoneID
		if ro.ElbHostedZoneID != "" {
			zoneId = ro.ElbHostedZoneID
		}
		at = &route53.AliasTarget{
			EvaluateTargetHealth: aws.Bool(true),
			HostedZoneId:         aws.String(zoneId),
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
		{
			Action: aws.String(action),
			ResourceRecordSet: &route53.ResourceRecordSet{
				Name: aws.String(fmt.Sprintf("%s%s", ro.TXTPrefix, ro.Hostname)),
				ResourceRecords: []*route53.ResourceRecord{
					{Value: aws.String("\"set by external-route53\"")},
				},
				SetIdentifier: aws.String(ro.Identifier),
				Weight:        aws.Int64(int64(ro.Weight)),
				HealthCheckId: healthCheckId,
				Type:          aws.String("TXT"),
				TTL:           aws.Int64(300),
			},
		},
	}
	logrus.Info(changes)
	_, err := d.client.ChangeResourceRecordSets(&route53.ChangeResourceRecordSetsInput{
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

func (d *Dns) validateRecordSetOpt(ro UpsertRecordSetOpt) error {
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
	if ok, err := d.hasValidTxtRecord(ro); err != nil || !ok {
		return errors.New("This record doesn't have valid txt record. it's possible to maintain from other system")
	}
	return nil
}

/**
The records created by this controller has TXT record for management.
Valid record is below:
  1. TXT record exists. if set, it has prefix ex: prefix-example.com for managing example.com record.
  2. TXT record has a value of the record's identifier. ex: uuid
*/
func (d *Dns) hasValidTxtRecord(ro UpsertRecordSetOpt) (bool, error) {
	txtname := fmt.Sprintf("%s%s", ro.TXTPrefix, ro.Hostname)
	out, err := d.client.ListResourceRecordSets(&route53.ListResourceRecordSetsInput{
		HostedZoneId:    aws.String(ro.HostedZoneID),
		StartRecordName: aws.String(txtname),
	})
	if err != nil {
		return false, err
	}
	ret := false
	contains := false
	if len(out.ResourceRecordSets) == 0 {
		return true, nil
	}
	for _, rs := range out.ResourceRecordSets {
		if domainEqual(ro.Hostname, *rs.Name) {
			contains = true
		}
		if domainEqual(txtname, *rs.Name) && *rs.SetIdentifier == ro.Identifier && *rs.Type == "TXT" {
			ret = true
		}
	}
	return ret || !contains, nil
}

func domainEqual(s1, s2 string) bool {
	return s1 == s2 || fmt.Sprintf("%s.", s1) == s2 || fmt.Sprintf("%s.", s2) == s1
}

func supportedType(t string) bool {
	// only A record is supported
	return t == "A"
}

func getElbCanonicalHostedZoneId(dnsName string) (zoneId *string, err error) {
	mySession := session.Must(session.NewSession())
	c := elbv2.New(mySession, aws.NewConfig().WithRegion("ap-northeast-1"))

	// perse Load Balancer name from dns record
	// ex: "xxx-yyy-0123456789-abcdefghijklmn.elb.ap-northeast-1.amazonaws.com" to xxx-yyy-0123456789
	n := strings.Split(strings.Split(dnsName, ".")[0], "-")
	lbName := strings.Join(n[:len(n)-1], "-")

	resp, err := c.DescribeLoadBalancers(&elbv2.DescribeLoadBalancersInput{
		Names: []*string{aws.String(lbName)},
	})
	if err != nil {
		return zoneId, err
	}
	if len(resp.LoadBalancers) == 0 {
		return zoneId, fmt.Errorf("ELB:\"%s\" not found", dnsName)
	}
	zoneId = resp.LoadBalancers[0].CanonicalHostedZoneId
	return zoneId, nil
}
