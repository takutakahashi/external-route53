package healthcheck

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/juju/errors"
	route53v1 "github.com/takutakahashi/external-route53/api/v1"
	r53client "github.com/takutakahashi/external-route53/pkg/client"
	"github.com/takutakahashi/external-route53/pkg/dns"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func EnsureResource(svc *corev1.Service) (*corev1.Service, error) {
	h, err := buildResource(svc)
	if err != nil {
		return nil, err
	}
	if h == nil {
		return nil, nil
	}
	c, err := r53client.New()
	if err != nil {
		return nil, err
	}
	ctx := context.TODO()
	nn := types.NamespacedName{
		Name:      h.Name,
		Namespace: h.Namespace,
	}
	if err := c.Get(ctx, nn, h); err == nil {
		h, err = buildResource(svc)
		if err != nil {
			return nil, err
		}
		if err := c.Update(ctx, h, &client.UpdateOptions{}); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err := c.Create(ctx, h, &client.CreateOptions{}); err != nil {
		return nil, err
	}
	for {
		if err := c.Get(ctx, nn, h); err != nil {
			return nil, err
		}
		if h.Status.ID != "" {
			svc.Annotations[dns.HealthCheckIdAnnotationKey] = h.Status.ID
			return svc, nil
		}
		time.Sleep(10 * time.Second)
	}
}

func buildResource(svc *corev1.Service) (*route53v1.HealthCheck, error) {
	if svc.Spec.Type == corev1.ServiceTypeExternalName {
		return nil, nil
	}
	if len(svc.Spec.Ports) == 0 {
		return nil, errors.New("no ports were found")
	}
	if len(svc.Status.LoadBalancer.Ingress) == 0 {
		return nil, errors.New("no loadbalancer IP was found")
	}
	p := svc.Spec.Ports[0].Port
	if svc.Spec.Type == corev1.ServiceTypeNodePort {
		p = svc.Spec.Ports[0].NodePort
	}
	h := route53v1.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svc.Name,
			Namespace: svc.Namespace,
		},
		Spec: route53v1.HealthCheckSpec{
			Enabled:  true,
			Invert:   false,
			Protocol: route53v1.ProtocolTCP,
			Port:     int(p),
			Endpoint: route53v1.HealthCheckEndpoint{
				Address: svc.Status.LoadBalancer.Ingress[0].IP,
			},
			FailureThreshold: 3,
			Features: route53v1.HealthCheckFeatures{
				FastInterval: true,
			},
		},
	}
	return &h, nil
}

func Ensure(h *route53v1.HealthCheck) (*route53v1.HealthCheck, error) {
	name := fmt.Sprintf("%s/%s", h.Namespace, h.Name)
	callerReference := fmt.Sprintf("%s/%s", name, h.ResourceVersion)
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
	id := h.Status.ID
	lout, err := r.ListHealthChecks(&route53.ListHealthChecksInput{})
	if err != nil {
		return nil, err
	}
	for _, res := range lout.HealthChecks {
		if *res.CallerReference == callerReference {
			id = *res.Id
		}
	}
	var resourcePath *string
	var enableSNI *bool
	if h.Spec.Protocol == route53v1.ProtocolTCP {
		resourcePath = nil
		enableSNI = nil
	} else {
		resourcePath = aws.String(h.Spec.Path)
		enableSNI = aws.Bool(true)
	}
	if id == "" {
		out, err := r.CreateHealthCheck(&route53.CreateHealthCheckInput{
			CallerReference: aws.String(callerReference),
			HealthCheckConfig: &route53.HealthCheckConfig{
				EnableSNI:                enableSNI,
				FailureThreshold:         aws.Int64(int64(h.Spec.FailureThreshold)),
				Port:                     aws.Int64(int64(h.Spec.Port)),
				FullyQualifiedDomainName: hostname,
				IPAddress:                ip,
				ResourcePath:             resourcePath,
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
			EnableSNI:                enableSNI,
			FailureThreshold:         aws.Int64(int64(h.Spec.FailureThreshold)),
			FullyQualifiedDomainName: hostname,
			IPAddress:                ip,
			ResourcePath:             resourcePath,
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
				Value: aws.String(name)},
		},
	})
	if err != nil {
		return nil, err
	}
	return h, nil
}

func Delete(h *route53v1.HealthCheck) (*route53v1.HealthCheck, error) {
	mySession := session.Must(session.NewSession())
	r := route53.New(mySession)
	_, err := r.DeleteHealthCheck(&route53.DeleteHealthCheckInput{
		HealthCheckId: aws.String(h.Status.ID),
	})
	if err != nil {
		return nil, err
	}
	h.Status.ID = ""
	return h, nil
}
