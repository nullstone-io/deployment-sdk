package ecs

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

const (
	VersionTagKey = "nullstone.io/version"
)

func UpdateTaskDefTagVersion(tags []ecstypes.Tag, version string) []ecstypes.Tag {
	for i, tag := range tags {
		if tag.Key != nil && *tag.Key == VersionTagKey {
			tags[i].Value = aws.String(version)
			return tags
		}
	}

	// If we didn't find the version tag, add it
	tags = append(tags, ecstypes.Tag{
		Key:   aws.String(VersionTagKey),
		Value: aws.String(version),
	})
	return tags
}
