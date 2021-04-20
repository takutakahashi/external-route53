package healthcheck

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	v1 "github.com/takutakahashi/external-route53/api/v1"
)

func Ensure(h *v1.HealthCheck) (*v1.HealthCheck, error) {
	mySession := session.Must(session.NewSession())
	r := route53.New(mySession)
	var ip, hostname *string = nil, nil
	if h.Spec.Endpoint.Address != "" {
		ip = aws.String(h.Spec.Endpoint.Address)
	}
	if h.Spec.Endpoint.Hostname != "" {
		hostname = aws.String(h.Spec.Endpoint.Hostname)
	}
	var requestInterval int64
	if h.Spec.Features.FastInterval {
		requestInterval = 10
	} else {
		requestInterval = 30
	}
	id := ""
	lout, err := r.ListHealthChecks(&route53.ListHealthChecksInput{})
	if err != nil {
		return nil, err
	}
	for _, res := range lout.HealthChecks {
		if *res.CallerReference == h.SelfLink {
			id = *res.Id
		}
	}
	if id == "" {

		out, err := r.CreateHealthCheck(&route53.CreateHealthCheckInput{
			CallerReference: aws.String(h.SelfLink),
			HealthCheckConfig: &route53.HealthCheckConfig{
				EnableSNI:                aws.Bool(true),
				FailureThreshold:         aws.Int64(int64(h.Spec.FailureThreshold)),
				FullyQualifiedDomainName: hostname,
				IPAddress:                ip,
				ResourcePath:             aws.String(h.Spec.Path),
				Type:                     aws.String(string(h.Spec.Protocol)),
				Inverted:                 aws.Bool(h.Spec.Invert),
				Disabled:                 aws.Bool(!h.Spec.Enabled),
				RequestInterval:          aws.Int64(requestInterval),
			},
		})
		if err != nil {
			return nil, err
		}
		h.Status.ID = *out.HealthCheck.Id
		return h, nil
	} else {

		out, err := r.UpdateHealthCheck(&route53.UpdateHealthCheckInput{
			HealthCheckId:            aws.String(id),
			EnableSNI:                aws.Bool(true),
			FailureThreshold:         aws.Int64(int64(h.Spec.FailureThreshold)),
			FullyQualifiedDomainName: hostname,
			IPAddress:                ip,
			ResourcePath:             aws.String(h.Spec.Path),
			Inverted:                 aws.Bool(h.Spec.Invert),
			Disabled:                 aws.Bool(!h.Spec.Enabled),
		})
		if err != nil {
			return nil, err
		}
		h.Status.ID = *out.HealthCheck.Id
	}
	_, err = r.ChangeTagsForResource(&route53.ChangeTagsForResourceInput{
		ResourceType: aws.String("healthcheck"),
		ResourceId:   aws.String(h.Status.ID),
		AddTags: []*route53.Tag{
			{
				Key:   aws.String("Name"),
				Value: aws.String(h.SelfLink)},
		},
	})
	if err != nil {
		return nil, err
	}
	return h, nil
}

func Delete(h v1.HealthCheck) error {
	return nil
}
