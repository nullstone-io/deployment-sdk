package ecs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

type ServiceHealth struct {
	LoadBalancers []ServiceLoadBalancerHealth
}

type ServiceLoadBalancerHealth struct {
	LoadBalancer ecstypes.LoadBalancer
	TargetGroups []elbv2types.TargetHealthDescription
}

func GetServiceHealth(ctx context.Context, infra Outputs) (ServiceHealth, error) {
	result := ServiceHealth{LoadBalancers: make([]ServiceLoadBalancerHealth, 0)}
	svc, err := GetService(ctx, infra)
	if err != nil {
		return result, err
	} else if svc == nil {
		return result, nil
	}

	for _, lb := range svc.LoadBalancers {
		slbh := ServiceLoadBalancerHealth{LoadBalancer: lb}
		if lb.TargetGroupArn != nil {
			healths, err := GetTargetGroupHealth(ctx, infra, *lb.TargetGroupArn)
			if err != nil {
				return result, err
			}
			slbh.TargetGroups = healths
		}
		result.LoadBalancers = append(result.LoadBalancers, slbh)
	}
	return result, nil
}

func (s ServiceHealth) FindByTargetId(targetId string) *elbv2types.TargetHealthDescription {
	for _, lb := range s.LoadBalancers {
		for _, tg := range lb.TargetGroups {
			if targetId == aws.ToString(tg.Target.Id) {
				return &tg
			}
		}
	}
	return nil
}
