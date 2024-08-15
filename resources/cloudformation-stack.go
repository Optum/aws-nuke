package resources

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationTypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/smithy-go"

	"github.com/rebuy-de/aws-nuke/v2/pkg/config"
	"github.com/rebuy-de/aws-nuke/v2/pkg/types"
	"github.com/sirupsen/logrus"
)

const (
	CLOUDFORMATION_MAX_DELETE_ATTEMPT        = 3
	MAX_WAIT_TIME                            = time.Duration(5) * time.Minute
	SERVICE_ROLE_NAME                 string = "nuke-service-role-CFS"
)

func init() {
	registerV2("CloudFormationStack", ListCloudFormationStacks)
}

func ListCloudFormationStacks(cfg *aws.Config) ([]Resource, error) {
	ctx := context.TODO()
	svc := cloudformation.NewFromConfig(*cfg)

	params := &cloudformation.DescribeStacksInput{}
	resources := make([]Resource, 0)

	for {
		resp, err := svc.DescribeStacks(ctx, params)
		if err != nil {
			return nil, err
		}
		for _, stack := range resp.Stacks {
			if stack.ParentId != nil && *stack.ParentId != "" {
				continue
			}
			resource := &CloudFormationStack{
				svc:               svc,
				iamSvc:            iam.NewFromConfig(*cfg),
				context:           ctx,
				stack:             stack,
				maxDeleteAttempts: CLOUDFORMATION_MAX_DELETE_ATTEMPT,
			}

			resources = append(resources, resource)
		}

		if resp.NextToken == nil {
			break
		}

		params.NextToken = resp.NextToken
	}

	return resources, nil
}

type CloudFormationStack struct {
	svc     *cloudformation.Client
	iamSvc  *iam.Client
	context context.Context

	stack             cloudformationTypes.Stack
	deleteRoleArn     *string
	maxDeleteAttempts int
	settings          config.Settings
}

func (cfs *CloudFormationStack) Settings(settings config.Settings) {
	cfs.settings = settings
}

func (cfs *CloudFormationStack) Remove() error {
	if cfs.settings.CloudFormationStack.EnableAutomaticRoleManagment {
		err := cfs.createServiceRoleArn()
		if err != nil {
			return err
		}
	}

	err := cfs.removeWithAttempts(0)

	if cfs.settings.CloudFormationStack.EnableAutomaticRoleManagment {
		delErr := cfs.deleteServiceRoleArn()
		if delErr != nil {
			return delErr
		}
	}

	return err
}

func (cfs *CloudFormationStack) removeWithAttempts(attempt int) error {
	if err := cfs.doRemove(); err != nil {
		logrus.Errorf("CloudFormationStack stackName=%s attempt=%d maxAttempts=%d delete failed: %s", *cfs.stack.StackName, attempt, cfs.maxDeleteAttempts, err.Error())
		var re *smithy.OperationError
		if errors.As(err, &re) && re.Error() == "Stack ["+*cfs.stack.StackName+"] cannot be deleted while TerminationProtection is enabled" {
			if cfs.settings.CloudFormationStack.DisableDeletionProtection {
				logrus.Infof("CloudFormationStack stackName=%s attempt=%d maxAttempts=%d updating termination protection", *cfs.stack.StackName, attempt, cfs.maxDeleteAttempts)
				_, err = cfs.svc.UpdateTerminationProtection(cfs.context, &cloudformation.UpdateTerminationProtectionInput{
					EnableTerminationProtection: aws.Bool(false),
					StackName:                   cfs.stack.StackName,
				})
				if err != nil {
					return err
				}
			} else {
				logrus.Warnf("CloudFormationStack stackName=%s attempt=%d maxAttempts=%d set feature flag to disable deletion protection", *cfs.stack.StackName, attempt, cfs.maxDeleteAttempts)
				return err
			}
		}
		if attempt >= cfs.maxDeleteAttempts {
			return errors.New("CFS might not be deleted after this run")
		} else {
			return cfs.removeWithAttempts(attempt + 1)
		}
	} else {
		return nil
	}
}

func (cfs *CloudFormationStack) doRemove() error {
	o, err := cfs.svc.DescribeStacks(cfs.context, &cloudformation.DescribeStacksInput{
		StackName: cfs.stack.StackName,
	})
	if err != nil {
		var re *smithy.OperationError
		if errors.As(err, &re) {
			logrus.Infof("CloudFormationStack stackName=%s no longer exists", *cfs.stack.StackName)
			return nil
		}
		return err
	}
	stack := o.Stacks[0]

	if stack.StackStatus == cloudformationTypes.StackStatusDeleteComplete {
		//stack already deleted, no need to re-delete
		return nil
	} else if stack.StackStatus == cloudformationTypes.StackStatusDeleteInProgress {
		logrus.Infof("CloudFormationStack stackName=%s delete in progress. Waiting", *cfs.stack.StackName)
		waiter := cloudformation.NewStackDeleteCompleteWaiter(cfs.svc)
		return waiter.Wait(cfs.context, &cloudformation.DescribeStacksInput{
			StackName: cfs.stack.StackName,
		}, MAX_WAIT_TIME)
	} else if stack.StackStatus == cloudformationTypes.StackStatusDeleteFailed {
		logrus.Infof("CloudFormationStack stackName=%s delete failed. Attempting to retain and delete stack", *cfs.stack.StackName)
		// This means the CFS has undeleteable resources.
		// In order to move on with nuking, we retain them in the deletion.
		retainableResources, err := cfs.svc.ListStackResources(cfs.context, &cloudformation.ListStackResourcesInput{
			StackName: cfs.stack.StackName,
		})
		if err != nil {
			return err
		}

		retain := make([]string, 0)

		for _, r := range retainableResources.StackResourceSummaries {
			if r.ResourceStatus != cloudformationTypes.ResourceStatusDeleteComplete {
				retain = append(retain, *r.LogicalResourceId)
			}
		}

		_, err = cfs.svc.DeleteStack(cfs.context, &cloudformation.DeleteStackInput{
			StackName:       cfs.stack.StackName,
			RetainResources: retain,
			RoleARN:         cfs.deleteRoleArn,
		})
		if err != nil {
			return err
		}
		waiter := cloudformation.NewStackDeleteCompleteWaiter(cfs.svc)
		return waiter.Wait(cfs.context, &cloudformation.DescribeStacksInput{
			StackName: cfs.stack.StackName,
		}, MAX_WAIT_TIME)
	} else {
		deleteWaiter := cloudformation.NewStackDeleteCompleteWaiter(cfs.svc)
		if err := cfs.waitForStackToStabilize(stack.StackStatus); err != nil {
			return err
		} else if _, err := cfs.svc.DeleteStack(cfs.context, &cloudformation.DeleteStackInput{
			StackName: cfs.stack.StackName,
			RoleARN:   cfs.deleteRoleArn,
		}); err != nil {
			return err
		} else if err := deleteWaiter.Wait(cfs.context, &cloudformation.DescribeStacksInput{
			StackName: cfs.stack.StackName,
		}, MAX_WAIT_TIME); err != nil {
			return err
		} else {
			return nil
		}
	}
}
func (cfs *CloudFormationStack) waitForStackToStabilize(currentStatus cloudformationTypes.StackStatus) error {
	switch currentStatus {
	case cloudformationTypes.StackStatusUpdateInProgress:
		fallthrough
	case cloudformationTypes.StackStatusUpdateRollbackCompleteCleanupInProgress:
		fallthrough
	case cloudformationTypes.StackStatusUpdateRollbackInProgress:
		logrus.Infof("CloudFormationStack stackName=%s update in progress. Waiting to stabilize", *cfs.stack.StackName)
		waiter := cloudformation.NewStackUpdateCompleteWaiter(cfs.svc)
		return waiter.Wait(cfs.context, &cloudformation.DescribeStacksInput{
			StackName: cfs.stack.StackName,
		}, MAX_WAIT_TIME)
	case cloudformationTypes.StackStatusCreateInProgress:
		fallthrough
	case cloudformationTypes.StackStatusRollbackInProgress:
		logrus.Infof("CloudFormationStack stackName=%s create in progress. Waiting to stabilize", *cfs.stack.StackName)
		waiter := cloudformation.NewStackCreateCompleteWaiter(cfs.svc)
		return waiter.Wait(cfs.context, &cloudformation.DescribeStacksInput{
			StackName: cfs.stack.StackName,
		}, MAX_WAIT_TIME)
	default:
		return nil
	}
}

func (cfs *CloudFormationStack) Properties() types.Properties {
	properties := types.NewProperties()
	properties.Set("Name", cfs.stack.StackName)
	properties.Set("CreationTime", cfs.stack.CreationTime.Format(time.RFC3339))
	if cfs.stack.LastUpdatedTime == nil {
		properties.Set("LastUpdatedTime", cfs.stack.CreationTime.Format(time.RFC3339))
	} else {
		properties.Set("LastUpdatedTime", cfs.stack.LastUpdatedTime.Format(time.RFC3339))
	}

	for _, tagValue := range cfs.stack.Tags {
		properties.SetTag(tagValue.Key, tagValue.Value)
	}

	return properties
}

func (cfs *CloudFormationStack) String() string {
	return *cfs.stack.StackName
}

func (cfs *CloudFormationStack) createServiceRoleArn() error {
	serviceRoleName := SERVICE_ROLE_NAME + "-" + *cfs.stack.StackName
	if len(serviceRoleName) > 64 {
		serviceRoleName = serviceRoleName[:64]
	}

	role, _ := cfs.iamSvc.GetRole(cfs.context, &iam.GetRoleInput{
		RoleName: &serviceRoleName,
	})
	if role != nil && role.Role != nil {
		// the role exists, the stack is ready for deletion
		return nil
	}
	fmt.Println("Creating role")
	params := iam.CreateRoleInput{
		AssumeRolePolicyDocument: aws.String("{\"Version\": \"2012-10-17\",\"Statement\": [{\"Effect\": \"Allow\",\"Principal\": {\"Service\": \"cloudformation.amazonaws.com\"},\"Action\": \"sts:AssumeRole\"}]}"),
		Description:              aws.String("A service role is required to delete a cloudformation stack for which a specific role was used which no longer exists"),
		RoleName:                 aws.String(serviceRoleName),
	}

	roleCreationOutput, err := cfs.iamSvc.CreateRole(cfs.context, &params)
	if err != nil {
		fmt.Println("Role creation failed")
		fmt.Println(err.Error())
		return err
	}
	cfs.deleteRoleArn = roleCreationOutput.Role.Arn
	fmt.Println("Deletion role arn: ", *cfs.deleteRoleArn)

	// Ensure role has permission to clean up resources created via the stack
	attachPolicyParams := &iam.AttachRolePolicyInput{
		PolicyArn: aws.String("arn:aws:iam::aws:policy/AdministratorAccess"),
		RoleName:  &serviceRoleName,
	}

	_, err = cfs.iamSvc.AttachRolePolicy(cfs.context, attachPolicyParams)
	if err != nil {
		fmt.Println("Policy attachment failed")
		fmt.Println(err.Error())
		return err
	}
	time.Sleep(1 * time.Second)
	return err
}

func (cfs *CloudFormationStack) deleteServiceRoleArn() error {
	serviceRoleName := SERVICE_ROLE_NAME + "-" + *cfs.stack.StackName
	params := iam.DeleteRoleInput{
		RoleName: &serviceRoleName,
	}

	_, err := cfs.iamSvc.DeleteRole(cfs.context, &params)
	return err
}
