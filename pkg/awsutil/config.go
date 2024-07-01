package awsutil

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

func (c *Credentials) NewConfig(region string) (*aws.Config, error) {
	if c.config == nil {
		if c.HasProfile() && c.HasKeys() {
			return nil, fmt.Errorf("you have to specify a profile or credentials for at least one region")
		}

		if c.HasKeys() {
			cfg, err := config.LoadDefaultConfig(context.TODO(),
				config.WithCredentialsProvider(
					credentials.NewStaticCredentialsProvider(
						strings.TrimSpace(c.AccessKeyID),
						strings.TrimSpace(c.SecretAccessKey),
						strings.TrimSpace(c.SessionToken),
					),
				),
			)
			if err != nil {
				return nil, err
			}
			c.config = &cfg
			return c.config, nil
		}

		profile := "default"
		if c.HasProfile() {
			profile = c.Profile
		}

		if region == GlobalRegionID {
			region = "aws-global"
		}

		cfg, err := config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(region),
			config.WithSharedConfigProfile(profile),
		)
		if err != nil {
			return nil, err
		}

		// if given a role to assume, overwrite the cfg credentials with assume role credentials
		if c.AssumeRoleArn != "" {
			stsSvc := sts.NewFromConfig(cfg)
			creds := stscreds.NewAssumeRoleProvider(stsSvc, c.AssumeRoleArn)
			cfg.Credentials = aws.NewCredentialsCache(creds)
		}

		c.config = &cfg
	} else if c.config.Region != region {
		c.config.Region = region
	}

	return c.config, nil
}
