package resources

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/rebuy-de/aws-nuke/v2/pkg/types"
)

type SageMakerAlgorithm struct {
	svc           *sagemaker.Client
	algorithmName *string
	context       context.Context
}

func init() {
	registerV2("SageMakerAlgorithm", ListSageMakerAlgorithms)
}

func ListSageMakerAlgorithms(cfg *aws.Config) ([]Resource, error) {

	ctx := context.TODO()
	svc := sagemaker.NewFromConfig(*cfg)
	resources := []Resource{}

	params := &sagemaker.ListAlgorithmsInput{
		MaxResults: aws.Int32(30),
	}

	for {
		resp, err := svc.ListAlgorithms(ctx, params)
		if err != nil {
			return nil, err
		}

		for _, algorithm := range resp.AlgorithmSummaryList {
			resources = append(resources, &SageMakerAlgorithm{
				svc:           svc,
				algorithmName: algorithm.AlgorithmName,
				context:       ctx,
			})
		}

		if resp.NextToken == nil {
			break
		}

		params.NextToken = resp.NextToken
	}

	return resources, nil
}

func (f *SageMakerAlgorithm) Remove() error {

	_, err := f.svc.DeleteAlgorithm(f.context, &sagemaker.DeleteAlgorithmInput{
		AlgorithmName: f.algorithmName,
	})

	return err
}

func (f *SageMakerAlgorithm) String() string {
	return *f.algorithmName
}

func (f *SageMakerAlgorithm) Properties() types.Properties {
	properties := types.NewProperties()
	properties.Set("AlgorithmName", f.algorithmName)
	return properties
}
