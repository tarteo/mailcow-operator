package mailcow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	securityprovider "github.com/oapi-codegen/oapi-codegen/v2/pkg/securityprovider"
)

type MailcowOKResponse struct {
	Type string `json:"type"`
	Msg  string `json:"msg"`
}

type MailcowOKResponseMsgArray struct {
	Type string `json:"type"`
	Msg  []any  `json:"msg"`
}

type MailcowBadRequestResponse struct {
	Msg  string `json:"msg"`
	Type string `json:"type"`
}

func checkMailcowResponse(response *http.Response, body []byte) error {
	if response.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("mailcow api: unauthorized")
	}

	if response.StatusCode == http.StatusBadRequest {
		var badRequest MailcowBadRequestResponse
		if err := json.Unmarshal(body, &badRequest); err != nil {
			return fmt.Errorf("mailcow api: failed to parse response (%s)", string(body))
		}
		return fmt.Errorf("mailcow api: bad request (%s)", badRequest.Msg)
	}

	// Unable to parse response if it's a get request that returns an object e.g. /get/domain/{id}
	// If it's an successful response, the message will be an array which leads to an error during unmarshal
	// So we just return nil if unmarshal fails for these
	// In some cases the response is not an array of messages and some cases it is, these can be any type

	var responses []MailcowOKResponse
	if err := json.Unmarshal(body, &responses); err == nil {
		for _, r := range responses {
			if r.Type == "danger" {
				return fmt.Errorf("mailcow api: %s", r.Msg)
			}
		}
	}

	var responseWithMessageArrays []MailcowOKResponseMsgArray
	if err := json.Unmarshal(body, &responseWithMessageArrays); err == nil {
		for _, r := range responseWithMessageArrays {
			if r.Type == "danger" {
				return fmt.Errorf("mailcow api: %s", fmt.Sprintf("%v", r.Msg))
			}
		}
	}

	return nil
}

type MailcowRequestDoer struct {
	Client *http.Client
}

func (d *MailcowRequestDoer) Do(req *http.Request) (*http.Response, error) {
	response, err := d.Client.Do(req)
	if err != nil {
		return response, err
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	err = response.Body.Close()
	if err != nil {
		return nil, err
	}
	err = checkMailcowResponse(response, body)
	response.Body = io.NopCloser(bytes.NewReader(body))
	return response, err
}

func NewCustomClientWithResponses(endpoint string, apiKey string) (*ClientWithResponses, error) {
	apiKeyAuth, err := securityprovider.NewSecurityProviderApiKey("header", "X-API-Key", apiKey)
	if err != nil {
		return nil, err
	}
	requestDoer := &MailcowRequestDoer{Client: &http.Client{}}
	client, err := NewClientWithResponses(endpoint, WithRequestEditorFn(apiKeyAuth.Intercept), WithHTTPClient(requestDoer))
	return client, err
}
