package resources

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudfront"
	"github.com/aws/aws-sdk-go/service/wafv2"
	"github.com/rebuy-de/aws-nuke/v2/pkg/types"
)

type WAFv2WebACL struct {
	svc       *wafv2.WAFV2
	ID        *string
	name      *string
	lockToken *string
	scope     *string
}

func init() {
	register("WAFv2WebACL", ListWAFv2WebACLs,
		mapCloudControl("AWS::WAFv2::WebACL"))
}

func ListWAFv2WebACLs(sess *session.Session) ([]Resource, error) {
	svc := wafv2.New(sess)
	resources := []Resource{}

	params := &wafv2.ListWebACLsInput{
		Limit: aws.Int64(50),
		Scope: aws.String("REGIONAL"),
	}

	output, err := getWebACLs(svc, params)
	if err != nil {
		return []Resource{}, err
	}

	resources = append(resources, output...)

	if *sess.Config.Region == "us-east-1" {
		params.Scope = aws.String("CLOUDFRONT")

		output, err := getWebACLs(svc, params)
		if err != nil {
			return []Resource{}, err
		}

		resources = append(resources, output...)
	}

	return resources, nil
}

func getWebACLs(svc *wafv2.WAFV2, params *wafv2.ListWebACLsInput) ([]Resource, error) {
	resources := []Resource{}
	for {
		resp, err := svc.ListWebACLs(params)
		if err != nil {
			return nil, err
		}

		for _, webACL := range resp.WebACLs {
			resources = append(resources, &WAFv2WebACL{
				svc:       svc,
				ID:        webACL.Id,
				name:      webACL.Name,
				lockToken: webACL.LockToken,
				scope:     params.Scope,
			})
		}

		if resp.NextMarker == nil {
			break
		}

		params.NextMarker = resp.NextMarker
	}
	return resources, nil
}

// List cloudfront distributions associated with the cloudfront   WebACL
func (f *WAFv2WebACL) ListAssociatedCloudfrontDistributions() ([]*string, error) {
	sess := session.Must(session.NewSession())
	cf := cloudfront.New(sess)
	params := &cloudfront.ListDistributionsByWebACLIdInput{
		WebACLId: f.ID,
	}

	var distributions []*string
	for {

		resp, err := cf.ListDistributionsByWebACLId(params)
		if err != nil {
			return nil, err
		}

		for _, distribution := range resp.DistributionList.Items {
			if *distribution.WebACLId == *f.ID {
				distributions = append(distributions, distribution.Id)
			}
		}

		if resp.DistributionList.NextMarker == nil {
			break
		}

		params.Marker = resp.DistributionList.NextMarker
	}
	return distributions, nil
}

func (f *WAFv2WebACL) RemoveAssociatedCloudfrontDistributions() error {

	distributions, err := f.ListAssociatedCloudfrontDistributions()
	if err != nil {
		return err
	}

	cf := cloudfront.New(session.Must(session.NewSession()))
	for _, distribution := range distributions {
		_, err := cf.UpdateDistribution(&cloudfront.UpdateDistributionInput{
			Id: distribution,
			DistributionConfig: &cloudfront.DistributionConfig{
				WebACLId: f.ID,
			},
			IfMatch: nil,
		})
		if err != nil {
			return err
		}
	}

	for _, distribution := range distributions {
		_, err := f.svc.DisassociateWebACL(&wafv2.DisassociateWebACLInput{
			ResourceArn: distribution,
		})
		if err != nil {
			return err
		}

	}
	return nil
}

func (f *WAFv2WebACL) RemoveAssociatedResources() error {
	err := f.RemoveAssociatedCloudfrontDistributions()
	if err != nil {
		return err
	}
	return nil
}

func UpdateDistribution(svc *cloudfront.CloudFront, distributionID *string, webACLID *string) error {
	_, err := svc.UpdateDistribution(&cloudfront.UpdateDistributionInput{
		Id: distributionID,
		DistributionConfig: &cloudfront.DistributionConfig{
			WebACLId: webACLID,
		},
		IfMatch: nil,
	})
	if err != nil {
		return err
	}
	return nil
}

func (f *WAFv2WebACL) Remove() error {
	err := f.RemoveAssociatedResources()
	if err != nil {
		return err
	}
	err = UpdateDistribution(cloudfront.New(session.Must(session.NewSession())), f.ID, nil)

	_, err = f.svc.DeleteWebACL(&wafv2.DeleteWebACLInput{
		Id:        f.ID,
		Name:      f.name,
		Scope:     f.scope,
		LockToken: f.lockToken,
	})
	if err != nil {
		return err
	}
	return err
}

func (f *WAFv2WebACL) String() string {
	return *f.ID
}

func (f *WAFv2WebACL) Properties() types.Properties {
	properties := types.NewProperties()

	properties.
		Set("ID", f.ID).
		Set("Name", f.name).
		Set("Scope", f.scope)
	return properties
}
