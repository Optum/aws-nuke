package resources

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sagemaker"
	"github.com/rebuy-de/aws-nuke/v2/pkg/types"
)

type SageMakerAlgorithm struct {
	svc           *sagemaker.SageMaker
	algorithmName *string
}

func init() {
	register("SageMakerAlgorithm", ListSageMakerAlgorithms)
}

func ListSageMakerAlgorithms(sess *session.Session) ([]Resource, error) {

	svc := sagemaker.New(sess)
	resources := []Resource{}

	params := &sagemaker.ListAlgorithmsInput{
		MaxResults: aws.Int64(30),
	}

	for {
		resp, err := svc.ListAlgorithms(params)
		if err != nil {
			return nil, err
		}

		for _, algorithm := range resp.AlgorithmSummaryList {
			resources = append(resources, &SageMakerAlgorithm{
				svc:           svc,
				algorithmName: algorithm.AlgorithmName,
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

	_, err := f.svc.DeleteAlgorithm(&sagemaker.DeleteAlgorithmInput{
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
