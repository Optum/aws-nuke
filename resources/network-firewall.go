package resources

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/networkfirewall"
	networkfirwallTypes "github.com/aws/aws-sdk-go-v2/service/networkfirewall/types"

	"github.com/rebuy-de/aws-nuke/v2/pkg/types"
)

type NetworkFirewall struct {
	svc *networkfirewall.Client

	firewall  networkfirwallTypes.FirewallMetadata
	logConfig *networkfirwallTypes.LoggingConfiguration
	tags      []networkfirwallTypes.Tag
}

func init() {
	registerV2("NetworkFirewall", ListNetworkFirewalls)
}

func ListNetworkFirewalls(cfg *aws.Config) ([]Resource, error) {
	ctx := context.TODO()
	svc := networkfirewall.NewFromConfig(*cfg)

	resources := []Resource{}

	params := &networkfirewall.ListFirewallsInput{
		MaxResults: aws.Int32(100),
	}

	for {
		resp, err := svc.ListFirewalls(ctx, params)
		if err != nil {
			return nil, err
		}

		for _, firewall := range resp.Firewalls {
			tagParams := &networkfirewall.ListTagsForResourceInput{
				ResourceArn: firewall.FirewallArn,
				MaxResults:  aws.Int32(100),
			}

			tags := []networkfirwallTypes.Tag{}
			for {
				tagResp, tagErr := svc.ListTagsForResource(ctx, tagParams)
				if tagErr != nil {
					return nil, tagErr
				}

				tags = append(tags, tagResp.Tags...)

				if tagResp.NextToken == nil {
					break
				}
				tagParams.NextToken = tagResp.NextToken
			}

			// logging configuration required to delete firewall
			logResp, err := svc.DescribeLoggingConfiguration(ctx, &networkfirewall.DescribeLoggingConfigurationInput{
				FirewallArn: firewall.FirewallArn,
			})
			if err != nil {
				return nil, err
			}

			resources = append(resources, &NetworkFirewall{
				svc:       svc,
				firewall:  firewall,
				logConfig: logResp.LoggingConfiguration,
				tags:      tags,
			})
		}
		if resp.NextToken == nil {
			break
		}

		params.NextToken = resp.NextToken
	}
	return resources, nil
}

func (i *NetworkFirewall) Remove() error {
	ctx := context.TODO()

	if i.logConfig != nil {
		for index := 1; index <= len(i.logConfig.LogDestinationConfigs); index++ {
			// aws forces to only remove one at a time
			_, err := i.svc.UpdateLoggingConfiguration(ctx, &networkfirewall.UpdateLoggingConfigurationInput{
				FirewallArn: i.firewall.FirewallArn,
				LoggingConfiguration: &networkfirwallTypes.LoggingConfiguration{
					LogDestinationConfigs: i.logConfig.LogDestinationConfigs[index:],
				},
			})
			if err != nil {
				return err
			}
		}
	}

	params := &networkfirewall.DeleteFirewallInput{
		FirewallArn: i.firewall.FirewallArn,
	}

	_, err := i.svc.DeleteFirewall(ctx, params)
	if err != nil {
		return err
	}

	return nil
}

func (i *NetworkFirewall) Properties() types.Properties {
	properties := types.NewProperties()
	properties.Set("ARN", *i.firewall.FirewallArn)
	properties.Set("Name", *i.firewall.FirewallName)
	properties.Set("Logging Configured", i.logConfig != nil)

	for _, tag := range i.tags {
		properties.SetTag(tag.Key, *tag.Value)
	}

	return properties
}

func (i *NetworkFirewall) String() string {
	return *i.firewall.FirewallArn
}
