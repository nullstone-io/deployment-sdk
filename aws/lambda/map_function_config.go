package lambda

import (
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

func MapFunctionConfig(retrieved *lambda.GetFunctionConfigurationOutput) *lambda.UpdateFunctionConfigurationInput {
	config := &lambda.UpdateFunctionConfigurationInput{
		FunctionName:      retrieved.FunctionName,
		DeadLetterConfig:  retrieved.DeadLetterConfig,
		Description:       retrieved.Description,
		EphemeralStorage:  retrieved.EphemeralStorage,
		FileSystemConfigs: retrieved.FileSystemConfigs,
		Handler:           retrieved.Handler,
		KMSKeyArn:         retrieved.KMSKeyArn,
		Layers:            make([]string, 0),
		MemorySize:        retrieved.MemorySize,
		RevisionId:        retrieved.RevisionId,
		Role:              retrieved.Role,
		Runtime:           retrieved.Runtime,
		Timeout:           retrieved.Timeout,
	}
	if retrieved.ImageConfigResponse != nil {
		config.ImageConfig = retrieved.ImageConfigResponse.ImageConfig
	}
	if retrieved.TracingConfig != nil {
		config.TracingConfig = &types.TracingConfig{Mode: retrieved.TracingConfig.Mode}
	}
	if retrieved.VpcConfig != nil {
		config.VpcConfig = &types.VpcConfig{
			SecurityGroupIds: retrieved.VpcConfig.SecurityGroupIds,
			SubnetIds:        retrieved.VpcConfig.SubnetIds,
		}
	}
	if retrieved.Environment != nil {
		config.Environment = &types.Environment{
			Variables: retrieved.Environment.Variables,
		}
	}
	for _, layer := range retrieved.Layers {
		config.Layers = append(config.Layers, *layer.Arn)
	}
	return config
}
