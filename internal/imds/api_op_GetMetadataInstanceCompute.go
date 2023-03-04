package imds

import (
	"context"
	"encoding/json"
	"net/http"
)

const getInstanceMetadataPath = "/metadata/instance/compute"

// GetInstanceIdentity retrieves an identity document describing an instance.
// Error is returned if the request fails or is unable to parse the response.
func (c *Client) GetInstanceMetadata(ctx context.Context, params *GetInstanceMetadataInput) (*GetMetadataInstanceOutput, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", c.options.Endpoint+getInstanceMetadataPath, nil)
	req.Header.Set("Metadata", "True")

	q := req.URL.Query()
	q.Add("format", c.options.Format)
	q.Add("api-version", c.options.ApiVersion)
	req.URL.RawQuery = q.Encode()

	resp, err := c.options.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	result, err := buildGetInstanceMetadataOutput(resp)
	if err != nil {
		return nil, err
	}

	out := result.(*GetMetadataInstanceOutput)
	return out, nil
}

// GetInstanceMetadataInput provides the input parameters for GetMetadataInstance operation.
type GetInstanceMetadataInput struct{}

// GetMetadataInstanceOutput provides the output parameters for GetMetadataInstance operation.
type GetMetadataInstanceOutput struct {
	InstanceIdentityDocument
}

func buildGetInstanceMetadataOutput(resp *http.Response) (v interface{}, err error) {
	output := &GetMetadataInstanceOutput{}
	if err = json.NewDecoder(resp.Body).Decode(&output.InstanceIdentityDocument); err != nil {
		return nil, err
	}

	return output, nil
}

// InstanceIdentityDocument provides the shape for unmarshaling an metadata instance document
type InstanceIdentityDocument struct {
	AzEnvironment     string `json:"azEnvironment,omitempty"`
	Location          string `json:"location,omitempty"`
	PlacementGroupID  string `json:"placementGroupId,omitempty"`
	ResourceGroupName string `json:"resourceGroupName,omitempty"`
	ResourceID        string `json:"resourceId,omitempty"`
	SubscriptionID    string `json:"subscriptionId,omitempty"`
	Version           string `json:"version,omitempty"`
	VMID              string `json:"vmId,omitempty"`
	Zone              string `json:"zone,omitempty"`
}
