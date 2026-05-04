package ecs

import (
	"strconv"
	"strings"
)

// parseClusterName extracts the cluster name from an ECS cluster ARN of the form
// arn:aws:ecs:<region>:<account-id>:cluster/<cluster-name>.
// Returns "" if the ARN is empty or malformed.
func parseClusterName(clusterArn string) string {
	if clusterArn == "" {
		return ""
	}
	idx := strings.LastIndex(clusterArn, "/")
	if idx < 0 {
		return ""
	}
	return clusterArn[idx+1:]
}

// parseTaskDefinition extracts the family and revision from a task-definition ARN of the form
// arn:aws:ecs:<region>:<account-id>:task-definition/<family>:<revision>.
// Returns ("", 0) if the ARN is empty or malformed.
func parseTaskDefinition(taskDefinitionArn string) (string, int32) {
	if taskDefinitionArn == "" {
		return "", 0
	}
	parts := strings.Split(taskDefinitionArn, ":")
	if len(parts) < 2 {
		return "", 0
	}
	familySeg := parts[len(parts)-2]
	revSeg := parts[len(parts)-1]
	slash := strings.LastIndex(familySeg, "/")
	if slash < 0 {
		return "", 0
	}
	family := familySeg[slash+1:]
	rev, err := strconv.ParseInt(revSeg, 10, 32)
	if err != nil {
		return family, 0
	}
	return family, int32(rev)
}
