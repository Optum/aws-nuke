package resources

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sagemaker"
	"github.com/rebuy-de/aws-nuke/v2/pkg/types"
)

type SageMakerTrainingJob struct {
	svc             *sagemaker.SageMaker
	trainingJobName *string
}

func init() {
	register("SageMakerTrainingJob", ListSageMakerTrainingJobs)
}

func ListSageMakerTrainingJobs(sess *session.Session) ([]Resource, error) {
	svc := sagemaker.New(sess)
	resources := []Resource{}

	params := &sagemaker.ListTrainingJobsInput{
		MaxResults: aws.Int64(30),
	}

	for {
		resp, err := svc.ListTrainingJobs(params)
		if err != nil {
			return nil, err
		}

		for _, trainingJob := range resp.TrainingJobSummaries {
			resources = append(resources, &SageMakerTrainingJob{
				svc:             svc,
				trainingJobName: trainingJob.TrainingJobName,
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

	_, err := f.svc.StopTrainingJob(&sagemaker.StopTrainingJobInput{
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
