package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// RegisterRoute POSTs a new route to the admin API at adminBaseURL
// (e.g. "http://localhost:1356"). Returns nil on 201 Created.
func RegisterRoute(adminBaseURL, hostname, backend string) error {
	body, err := json.Marshal(addRouteRequest{Hostname: hostname, Backend: backend})
	if err != nil {
		return err
	}

	endpoint, err := url.JoinPath(adminBaseURL, "routes")
	if err != nil {
		return fmt.Errorf("build register URL: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		return nil
	}

	b, _ := io.ReadAll(resp.Body)
	if msg := parseJSONErrorBody(b); msg != "" {
		return fmt.Errorf("register route: %s: %s", resp.Status, msg)
	}
	return fmt.Errorf("register route: %s", resp.Status)
}

func parseJSONErrorBody(body []byte) string {
	var er jsonErrorResponse
	if json.Unmarshal(body, &er) == nil && er.Error != "" {
		return er.Error
	}
	return ""
}

// DeregisterRoute removes a route by hostname via the admin API at
// adminBaseURL (e.g. "http://localhost:1356"). Returns nil on 204 No Content.
func DeregisterRoute(adminBaseURL, hostname string) error {
	endpoint, err := url.JoinPath(adminBaseURL, "routes", url.PathEscape(hostname))
	if err != nil {
		return fmt.Errorf("build deregister URL: %w", err)
	}

	req, err := http.NewRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	b, _ := io.ReadAll(resp.Body)
	if msg := parseJSONErrorBody(b); msg != "" {
		return fmt.Errorf("deregister route: %s: %s", resp.Status, msg)
	}
	return fmt.Errorf("deregister route: %s", resp.Status)
}
