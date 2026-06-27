package xwatermill

import (
	"context"
	"fmt"
	"strings"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-aws/sns"
	"github.com/ThreeDotsLabs/watermill-aws/sqs"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
)

func LoadAWSConfig(ctx context.Context, opts ...func(*awsconfig.LoadOptions) error) (aws.Config, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
	}
	return cfg, nil
}

type SNSTopicResolver map[string]string

func (r SNSTopicResolver) ResolveTopic(_ context.Context, topic string) (sns.TopicArn, error) {
	arn, ok := r[topic]
	if !ok {
		return "", fmt.Errorf("unknown topic %s", topic)
	}
	return sns.TopicArn(arn), nil
}

func NewSNSPublisher(awsCfg aws.Config, topicResolver sns.TopicResolver, logger watermill.LoggerAdapter) (*sns.Publisher, error) {
	pub, err := sns.NewPublisher(sns.PublisherConfig{
		AWSConfig:     awsCfg,
		TopicResolver: topicResolver,
	}, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create SNS publisher: %w", err)
	}
	return pub, nil
}

func NewSNSSQSSubscriber(awsCfg aws.Config, topicResolver sns.TopicResolver, consumerGroup string, logger watermill.LoggerAdapter) (*sns.Subscriber, error) {
	snsConfig := sns.SubscriberConfig{
		AWSConfig:     awsCfg,
		TopicResolver: topicResolver,
		GenerateSqsQueueName: func(_ context.Context, snsTopic sns.TopicArn) (string, error) {
			topicName, err := sns.ExtractTopicNameFromTopicArn(snsTopic)
			if err != nil {
				return "", fmt.Errorf("failed to extract topic name from ARN: %w", err)
			}
			if baseName, ok := strings.CutSuffix(string(topicName), ".fifo"); ok {
				return fmt.Sprintf("%s-%s.fifo", baseName, consumerGroup), nil
			}
			return fmt.Sprintf("%s-%s", topicName, consumerGroup), nil
		},
	}

	sqsConfig := sqs.SubscriberConfig{
		AWSConfig: awsCfg,
	}

	sub, err := sns.NewSubscriber(snsConfig, sqsConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create SNS/SQS subscriber: %w", err)
	}
	return sub, nil
}
