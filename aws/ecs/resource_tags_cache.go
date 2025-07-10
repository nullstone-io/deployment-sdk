package ecs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
	"sync"
)

type ResourceTagsCache struct {
	Infra  Outputs
	client *ecs.Client
	mu     sync.Mutex
	once   sync.Once
	cache  map[string]map[string]string
}

func (c *ResourceTagsCache) init() {
	c.once.Do(func() {
		c.client = ecs.NewFromConfig(nsaws.NewConfig(c.Infra.Deployer, c.Infra.Region))
		c.cache = map[string]map[string]string{}
	})
}

func (c *ResourceTagsCache) Get(ctx context.Context, resourceArn string, key string) (string, error) {
	tags, err := c.GetAll(ctx, resourceArn)
	if err != nil {
		return "", err
	}
	return tags[key], nil
}

func (c *ResourceTagsCache) GetAll(ctx context.Context, resourceArn string) (map[string]string, error) {
	c.init()
	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.cache[resourceArn]; ok {
		return existing, nil
	}

	list, err := c.get(ctx, resourceArn)
	if err != nil {
		return nil, err
	}
	tags := map[string]string{}
	for _, tag := range list {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	c.cache[resourceArn] = tags
	return tags, nil
}

func (c *ResourceTagsCache) get(ctx context.Context, resourceArn string) ([]ecstypes.Tag, error) {
	out, err := c.client.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{
		ResourceArn: aws.String(resourceArn),
	})
	if err != nil {
		return nil, err
	}
	return out.Tags, nil
}
