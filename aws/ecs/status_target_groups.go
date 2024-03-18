package ecs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

type StatusTargetGroup struct {
	TargetGroupArn string
	// Targets indexes on TargetId
	Targets map[string]StatusTarget
}

func (g *StatusTargetGroup) RefreshHealth(ctx context.Context, infra Outputs) error {
	tgh, err := GetTargetGroupHealth(ctx, infra, g.TargetGroupArn)
	if err != nil {
		return err
	}

	targets := map[string]StatusTarget{}
	for _, thd := range tgh {
		st := StatusTargetFromEcs(thd)
		targets[st.TargetId] = st
	}
	g.Targets = targets
	return nil
}

type StatusTarget struct {
	// TargetId is unique and different based on attached resources
	// - EC2: Instance ID
	// - ECS: IP Address
	// - Lambda: Lambda function ARN
	// - ALB: Load Balancer Target Arn
	TargetId               string
	TargetAvailabilityZone string
	TargetPort             int32
	HealthCheckPort        string
	HealthDescription      string
	HealthReason           types.TargetHealthReasonEnum
	HealthState            types.TargetHealthStateEnum
}

func StatusTargetFromEcs(thd types.TargetHealthDescription) StatusTarget {
	var target types.TargetDescription
	if thd.Target != nil {
		target = *thd.Target
	}
	var health types.TargetHealth
	if thd.TargetHealth != nil {
		health = *thd.TargetHealth
	}
	return StatusTarget{
		HealthCheckPort:        aws.ToString(thd.HealthCheckPort),
		TargetId:               aws.ToString(target.Id),
		TargetAvailabilityZone: aws.ToString(target.AvailabilityZone),
		TargetPort:             aws.ToInt32(target.Port),
		HealthDescription:      aws.ToString(health.Description),
		HealthReason:           health.Reason,
		HealthState:            health.State,
	}
}
