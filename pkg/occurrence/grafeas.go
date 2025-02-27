package occurrence

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/go-logr/logr"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
	project "github.com/grafeas/grafeas/proto/v1beta1/project_go_proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type grafeasClient struct {
	log                logr.Logger
	client             grafeas.GrafeasV1Beta1Client
	projectClient      project.ProjectsClient
	projectID          string
	projectInitialized bool
}

// GrafeasClient handle into grafeas
type GrafeasClient interface {
	Creator
	Lister
}

// NewGrafeasClient creates a new client
func NewGrafeasClient(log logr.Logger, tlsConfig *tls.Config, endpoint string) (GrafeasClient, error) {
	log.Info("Using Grafeas endpoint", "Endpoint", endpoint)

	grpcDialOption := grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))

	conn, err := grpc.Dial(endpoint, grpcDialOption)
	if err != nil {
		return nil, err
	}

	client := grafeas.NewGrafeasV1Beta1Client(conn)
	projectClient := project.NewProjectsClient(conn)
	c := &grafeasClient{
		log,
		client,
		projectClient,
		"projects/rode",
		false,
	}

	return c, nil
}

// ListOccurrences will get the occurence for a resource
func (c *grafeasClient) ListOccurrences(ctx context.Context, resourceURI string) (*grafeas.ListOccurrencesResponse, error) {
	c.log.Info("Get occurrences for resource", "resouceURI", resourceURI)

	resp, err := c.client.ListOccurrences(ctx, &grafeas.ListOccurrencesRequest{
		Parent:   c.projectID,
		Filter:   fmt.Sprintf("resource.uri = '%s'", resourceURI),
		PageSize: 1000,
	})

	if err != nil {
		return nil, err
	}

	// TODO: remove this hack...grafeas doesn't support filter yet
	occurrences := make([]*grafeas.Occurrence, 0)
	for _, o := range resp.GetOccurrences() {
		if o.Resource.Uri == resourceURI {
			occurrences = append(occurrences, o)
		}
	}

	return &grafeas.ListOccurrencesResponse{
		Occurrences: occurrences,
	}, nil
}

// CreateOccurrences will save the occurence in grafeas
func (c *grafeasClient) CreateOccurrences(ctx context.Context, occurrences ...*grafeas.Occurrence) error {
	if len(occurrences) == 0 {
		return nil
	}

	err := c.initProject(ctx)
	if err != nil {
		return err
	}

	_, err = c.client.BatchCreateOccurrences(ctx, &grafeas.BatchCreateOccurrencesRequest{
		Occurrences: occurrences,
		Parent:      c.projectID,
	})
	return err
}

func (c *grafeasClient) initProject(ctx context.Context) error {
	if c.projectInitialized {
		return nil
	}

	c.log.Info("Fetching project", "projectID", c.projectID)
	_, err := c.projectClient.GetProject(ctx, &project.GetProjectRequest{
		Name: c.projectID,
	})
	if err != nil && status.Code(err) == codes.NotFound {
		c.log.Info("Creating project", "ProjectID", c.projectID)
		_, err = c.projectClient.CreateProject(ctx, &project.CreateProjectRequest{
			Project: &project.Project{
				Name: c.projectID,
			},
		})
	}
	c.projectInitialized = (err == nil)
	return err
}
