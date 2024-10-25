package resources

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/rebuy-de/aws-nuke/v2/pkg/types"
)

type SageMakerTrainingJob struct {
	svc             *sagemaker.Client
	trainingJobName *string
	context         context.Context
}

func init() {
	registerV2("SageMakerTrainingJob", ListSageMakerTrainingJobs)
}

func ListSageMakerTrainingJobs(cfg *aws.Config) ([]Resource, error) {
	ctx := context.TODO()
	svc := sagemaker.NewFromConfig(*cfg)
	resources := []Resource{}

	params := &sagemaker.ListTrainingJobsInput{
		MaxResults: aws.Int32(30),
	}

	for {
		resp, err := svc.ListTrainingJobs(ctx, params)
		if err != nil {
			return nil, err
		}

		for _, trainingJob := range resp.TrainingJobSummaries {
			resources = append(resources, &SageMakerTrainingJob{
				svc:             svc,
				trainingJobName: trainingJob.TrainingJobName,
				context:         ctx,
			})
		}

		if resp.NextToken == nil {
			break
		}

		params.NextToken = resp.NextToken
	}

	return resources, nil
}

func (f *SageMakerTrainingJob) Remove() error {

	_, err := f.svc.StopTrainingJob(f.context, &sagemaker.StopTrainingJobInput{
		TrainingJobName: f.trainingJobName,
	})

	return err
}

func (f *SageMakerTrainingJob) String() string {
	return *f.trainingJobName
}

func (f *SageMakerTrainingJob) Properties() types.Properties {
	properties := types.NewProperties()
	properties.Set("TrainingJobName", f.trainingJobName)
	return properties
}
