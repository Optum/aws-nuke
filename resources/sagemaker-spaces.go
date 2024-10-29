package resources

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/rebuy-de/aws-nuke/v2/pkg/types"
)

type SageMakerSpace struct {
	svc       *sagemaker.Client
	spaceName *string
	domainID  *string
	context   context.Context
}

func init() {
	registerV2("SageMakerSpaces", ListSageMakerSpaces)
}

func ListSageMakerSpaces(cfg *aws.Config) ([]Resource, error) {
	ctx := context.TODO()
	svc := sagemaker.NewFromConfig(*cfg)
	resources := []Resource{}

	params := &sagemaker.ListSpacesInput{
		MaxResults: aws.Int32(30),
	}

	for {
		resp, err := svc.ListSpaces(ctx, params)
		if err != nil {
			return nil, err
		}

		for _, space := range resp.Spaces {
			resources = append(resources, &SageMakerSpace{
				svc:       svc,
				spaceName: space.SpaceName,
				domainID:  space.DomainId,
				context:   ctx,
			})
			deleteAppsInSpace(ctx, cfg, *space.DomainId, *space.SpaceName, svc)
		}

		if resp.NextToken == nil {
			break
		}

		params.NextToken = resp.NextToken
	}

	return resources, nil
}

func (f *SageMakerSpace) Remove() error {

	_, err := f.svc.DeleteSpace(f.context, &sagemaker.DeleteSpaceInput{
		SpaceName: f.spaceName,
		DomainId:  f.domainID,
	})

	return err
}

func (f *SageMakerSpace) String() string {
	return *f.spaceName
}

func deleteAppsInSpace(ctx context.Context, cfg *aws.Config, DomainId string, SpaceName string, svc *sagemaker.Client) error {

	input := &sagemaker.ListAppsInput{
		DomainIdEquals:  &DomainId,
		SpaceNameEquals: &SpaceName,
		MaxResults:      aws.Int32(30),
	}
	svc = sagemaker.NewFromConfig(*cfg)
	for {
		resp, err := svc.ListApps(ctx, input)
		if err != nil {
			return err
		}

		for _, app := range resp.Apps {
			_, err := svc.DeleteApp(ctx, &sagemaker.DeleteAppInput{
				DomainId:  app.DomainId,
				AppName:   app.AppName,
				AppType:   app.AppType,
				SpaceName: app.SpaceName,
			})
			if err != nil {
				return err
			}
		}

		if resp.NextToken == nil {
			break
		}

		input.NextToken = resp.NextToken
	}

	return nil

}

func (f *SageMakerSpace) Properties() types.Properties {
	properties := types.NewProperties()
	properties.Set("SpaceName", f.spaceName)
	properties.Set("DomainId", f.domainID)
	return properties
}
