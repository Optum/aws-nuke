package resources

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/healthlake"
)

type HealthlakeDatastore struct {
        svc  *healthlake.HealthLake
        datastoreId *string 
}

func init() {
	register("HealthlakeDatastore", ListHealthlakeDatastores)
}

func ListHealthlakeDatastores(sess *session.Session) ([]Resource, error) {
	svc := healthlake.New(sess)

	resources := make([]Resource, 0)
        params := &healthlake.ListFHIRDatastoresInput{}

        for {
                output, err := svc.ListFHIRDatastores(params)
                if err != nil {
                        return nil, err
                }

                for _, datastore := range output.DatastorePropertiesList {
                        if *datastore.DatastoreStatus != "DELETED" {
                            resources = append(resources, &HealthlakeDatastore{
                                    svc:  svc,
                                    datastoreId: datastore.DatastoreId,
                            })
                        }
                }

                if output.NextToken == nil {
                        break
                }

                params.NextToken = output.NextToken
        }

        return resources, nil
}

func (e *HealthlakeDatastore) Remove() error {
	params := &healthlake.DeleteFHIRDatastoreInput{
		DatastoreId: e.datastoreId,
	}

	_, err := e.svc.DeleteFHIRDatastore(params)
	if err != nil {
		return err
	}

	return nil
}

func (e *HealthlakeDatastore) String() string {
    return *e.datastoreId
}
