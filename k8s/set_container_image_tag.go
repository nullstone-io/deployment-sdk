package k8s

import (
	"github.com/nullstone-io/deployment-sdk/docker"
	core_v1 "k8s.io/api/core/v1"
)

func SetContainerImageTag(container *core_v1.Container, imageTag string) {
	existingImageUrl := docker.ParseImageUrl(container.Image)
	existingImageUrl.Digest = ""
	existingImageUrl.Tag = imageTag
	container.Image = existingImageUrl.String()
}
