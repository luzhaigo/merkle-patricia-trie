package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func RegisterRoute(adminAddr, hostname, backend string) error {
	body, err := json.Marshal(addRouteRequest{Hostname: hostname, Backend: backend})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "http://"+adminAddr+"/routes", bytes.NewReader(body))
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
	msg := parseJSONErrorBody(b)
	if msg != "" {
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

// DeregisterRoute removes a route by hostname via the admin API.
func DeregisterRoute(adminAddr, hostname string) error {
	u := "http://" + adminAddr + "/routes/" + url.PathEscape(hostname)
	req, err := http.NewRequest("DELETE", u, nil)
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
