package resources

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sagemaker"
	"github.com/rebuy-de/aws-nuke/v2/pkg/types"
)

type SageMakerSpace struct {
	svc       *sagemaker.SageMaker
	spaceName *string
	domainID  *string
}

type SageMakerAppDelete struct {
	svc       *sagemaker.SageMaker
	appName   *string
	domainID  *string
	appType   *string
	spaceName *string
}

func init() {
	register("SageMakerSpaces", ListSageMakerSpaces)
}

func ListSageMakerSpaces(sess *session.Session) ([]Resource, error) {
	svc := sagemaker.New(sess)
	resources := []Resource{}

	params := &sagemaker.ListSpacesInput{
		MaxResults: aws.Int64(30),
	}

	for {
		resp, err := svc.ListSpaces(params)
		if err != nil {
			return nil, err
		}

		for _, space := range resp.Spaces {
			resources = append(resources, &SageMakerSpace{
				svc:       svc,
				spaceName: space.SpaceName,
				domainID:  space.DomainId,
			})
			deleteAppsInSpace(sess, *space.DomainId, *space.SpaceName, svc)
		}

		if resp.NextToken == nil {
			break
		}

		params.NextToken = resp.NextToken
	}

	return resources, nil
}

func (f *SageMakerSpace) Remove() error {

	_, err := f.svc.DeleteSpace(&sagemaker.DeleteSpaceInput{
		SpaceName: f.spaceName,
		DomainId:  f.domainID,
	})

	return err
}

func (f *SageMakerSpace) String() string {
	return *f.spaceName
}

func deleteAppsInSpace(sess *session.Session, DomainId string, SpaceName string, svc *sagemaker.SageMaker) error {
	input := &sagemaker.ListAppsInput{
		DomainIdEquals:  &DomainId,
		SpaceNameEquals: &SpaceName,
		MaxResults:      aws.Int64(30),
	}
	svc = sagemaker.New(sess)
	for {
		resp, err := svc.ListApps(input)
		if err != nil {
			return err
		}

		for _, app := range resp.Apps {
			_, err := svc.DeleteApp(&sagemaker.DeleteAppInput{
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
