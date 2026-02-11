package monitoring

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// CloudWatchMonitor publishes custom security metrics to CloudWatch.
// In development mode, metrics are logged to stdout instead.
type CloudWatchMonitor struct {
	client *cloudwatch.Client
	isDev  bool
}

var (
	instance *CloudWatchMonitor
	once     sync.Once
	initErr  error
)

// NewCloudWatchMonitor creates or returns the singleton CloudWatch monitor.
// Pass isProduction=true to enable real CloudWatch publishing.
func NewCloudWatchMonitor(isProduction bool) (*CloudWatchMonitor, error) {
	once.Do(func() {
		if !isProduction {
			instance = &CloudWatchMonitor{isDev: true}
			return
		}

		cfg, err := awsconfig.LoadDefaultConfig(context.Background())
		if err != nil {
			initErr = err
			return
		}

		instance = &CloudWatchMonitor{
			client: cloudwatch.NewFromConfig(cfg),
			isDev:  false,
		}
	})

	if initErr != nil {
		return nil, initErr
	}
	return instance, nil
}

// PublishSecurityMetric publishes a security event metric to CloudWatch.
func (m *CloudWatchMonitor) PublishSecurityMetric(eventType string, success bool) {
	if m.isDev {
		log.Printf("[CLOUDWATCH_DEV] Security metric: %s success=%v", eventType, success)
		return
	}

	value := 1.0

	_, err := m.client.PutMetricData(context.Background(), &cloudwatch.PutMetricDataInput{
		Namespace: aws.String("StreamMaxing/Security"),
		MetricData: []types.MetricDatum{
			{
				MetricName: aws.String(eventType),
				Value:      aws.Float64(value),
				Unit:       types.StandardUnitCount,
				Timestamp:  aws.Time(time.Now()),
				Dimensions: []types.Dimension{
					{
						Name:  aws.String("Success"),
						Value: aws.String(boolToString(success)),
					},
				},
			},
		},
	})
	if err != nil {
		log.Printf("[CLOUDWATCH_ERROR] Failed to publish metric %s: %v", eventType, err)
	}
}

// PublishRateLimitMetric publishes a rate limit event metric.
func (m *CloudWatchMonitor) PublishRateLimitMetric() {
	if m.isDev {
		log.Printf("[CLOUDWATCH_DEV] Rate limit exceeded metric")
		return
	}

	_, err := m.client.PutMetricData(context.Background(), &cloudwatch.PutMetricDataInput{
		Namespace: aws.String("StreamMaxing/RateLimit"),
		MetricData: []types.MetricDatum{
			{
				MetricName: aws.String("RateLimitExceeded"),
				Value:      aws.Float64(1.0),
				Unit:       types.StandardUnitCount,
				Timestamp:  aws.Time(time.Now()),
			},
		},
	})
	if err != nil {
		log.Printf("[CLOUDWATCH_ERROR] Failed to publish rate limit metric: %v", err)
	}
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
