package ecs

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type StatusLoadBalancers []StatusLoadBalancer

func StatusLoadBalancersFromEcsService(service *ecstypes.Service) StatusLoadBalancers {
	lbs := StatusLoadBalancers{}
	if service == nil {
		return lbs
	}
	for _, lb := range service.LoadBalancers {
		lbs = append(lbs, StatusLoadBalancerFromEcs(lb))
	}
	return lbs
}

func (s StatusLoadBalancers) RefreshHealth(ctx context.Context, infra Outputs) error {
	for i, lb := range s {
		updated := lb
		if err := updated.RefreshHealth(ctx, infra); err != nil {
			return fmt.Errorf("error refreshing load balancer health: %w", err)
		}
		s[i] = updated
	}
	return nil
}

func (s StatusLoadBalancers) FindTargetsById(id string) []StatusTarget {
	targets := make([]StatusTarget, 0)
	for _, lb := range s {
		if target, ok := lb.TargetGroup.Targets[id]; ok {
			targets = append(targets, target)
		}
	}
	return targets
}

type StatusLoadBalancer struct {
	Name          string
	ContainerName string
	ContainerPort int32
	TargetGroup   *StatusTargetGroup
}

func StatusLoadBalancerFromEcs(lb ecstypes.LoadBalancer) StatusLoadBalancer {
	return StatusLoadBalancer{
		Name:          aws.ToString(lb.LoadBalancerName),
		ContainerName: aws.ToString(lb.ContainerName),
		ContainerPort: aws.ToInt32(lb.ContainerPort),
		TargetGroup: &StatusTargetGroup{
			TargetGroupArn: aws.ToString(lb.TargetGroupArn),
			Targets:        map[string]StatusTarget{},
		},
	}
}

func (l *StatusLoadBalancer) RefreshHealth(ctx context.Context, infra Outputs) error {
	return l.TargetGroup.RefreshHealth(ctx, infra)
}
