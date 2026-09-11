package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
)

const (
	infomaniakBaseURL = "https://api.infomaniak.com"
)

// InfomaniakAPI is a basic implementation of an API client to api.infomaniak.com
// It implements only the methods required for the ACME Challenge
type InfomaniakAPI struct {
	apiToken string
	baseURL  string
}

// ErrorResponse defines the error response format, as described here https://api.infomaniak.com/doc#home
type ErrorResponse struct {
	Code        string          `json:"code"`
	Description string          `json:"description,omitempty"`
	Context     map[string]any  `json:"context,omitempty"`
	Errors      []ErrorResponse `json:"errors,omitempty"`
}

// APIError represents an error returned by the Infomaniak API
type APIError struct {
	Code        string
	Description string
	Context     map[string]any
}

func (e *APIError) Error() string {
	msg := e.Code
	if e.Description != "" {
		msg += ": " + e.Description
	}
	if len(e.Context) > 0 {
		msg += fmt.Sprintf(" (context: %v)", e.Context)
	}
	return msg
}

// InfomaniakAPIResponse defines the generic response format, as described here https://api.infomaniak.com/doc#home
type InfomaniakAPIResponse struct {
	Result      string           `json:"result"`
	Data        *json.RawMessage `json:"data,omitempty"`
	ErrResponse ErrorResponse    `json:"error,omitempty"`
}

// NewInfomaniakAPI creates a new infomaniak API client
func NewInfomaniakAPI(apiToken string) *InfomaniakAPI {
	return &InfomaniakAPI{
		apiToken: apiToken,
		baseURL:  infomaniakBaseURL,
	}
}

// ErrZoneNotFound
var ErrZoneNotFound = errors.New("zone not found")

// GetZoneByName returns the name of the zone matching the given name,
// walking up the domain labels until a zone is found
func (ik *InfomaniakAPI) GetZoneByName(name string) (string, error) {
	klog.V(4).Infof("Getting zone matching `%s`", name)

	// remove trailing . if present
	if strings.HasSuffix(name, ".") {
		name = name[:len(name)-1]
	}

	// Try to find the most specific zone
	// starts with the FQDN, then remove each left label until we have a match
	for {
		i := strings.Index(name, ".")
		if i == -1 {
			break
		}

		exists, err := ik.zoneExists(name)
		if err != nil {
			return "", err
		}
		if exists {
			klog.V(4).Infof("Zone `%s` found", name)
			return name, nil
		}

		klog.V(4).Infof("Zone `%s` not found, trying with `%s`", name, name[i+1:])
		name = name[i+1:]
	}

	return "", ErrZoneNotFound
}

// zoneExists checks if a zone exists in the account
func (ik *InfomaniakAPI) zoneExists(zone string) (bool, error) {
	resp, err := ik.get(fmt.Sprintf("/2/zones/%s/exists", url.PathEscape(zone)), nil)
	if err != nil {
		// a missing zone is reported as a 404 object_not_found error envelope
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Code == "object_not_found" {
			return false, nil
		}
		return false, err
	}

	if resp.Data == nil {
		return false, fmt.Errorf("no data in response")
	}

	var exists bool
	if err := json.Unmarshal(*resp.Data, &exists); err != nil {
		return false, fmt.Errorf("expected boolean, got: %v", string(*resp.Data))
	}

	return exists, nil
}

// request builds the raw request
func (ik *InfomaniakAPI) request(method, path string, body io.Reader) (*InfomaniakAPIResponse, error) {
	if path[0] != '/' {
		path = "/" + path
	}
	requestURL := ik.baseURL + path

	client := &http.Client{}

	req, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+ik.apiToken)
	req.Header.Set("Content-Type", "application/json")

	klog.V(6).Infof("%s %s", method, requestURL)
	rawResp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer rawResp.Body.Close()

	var resp InfomaniakAPIResponse
	if err := json.NewDecoder(rawResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("%s %s response parsing error: %v", method, path, err)
	}

	rawJSON, _ := json.Marshal(resp)
	klog.V(8).Infof("Response status: `%s` json response: `%v`", rawResp.Status, string(rawJSON))

	if resp.Result != "success" {
		return nil, fmt.Errorf("%s %s failed: %w", method, path, &APIError{
			Code:        resp.ErrResponse.Code,
			Description: resp.ErrResponse.Description,
			Context:     resp.ErrResponse.Context,
		})
	}

	return &resp, nil
}

// get is a helper to build a bare GET request
func (ik *InfomaniakAPI) get(path string, params url.Values) (*InfomaniakAPIResponse, error) {
	base, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	if params != nil {
		base.RawQuery = params.Encode()
	}
	return ik.request("GET", base.String(), nil)
}

// get is a helper to build a bare POST request
func (ik *InfomaniakAPI) post(path string, body io.Reader) (*InfomaniakAPIResponse, error) {
	return ik.request("POST", path, body)
}

// get is a helper to build a bare PUT request
func (ik *InfomaniakAPI) put(path string, body io.Reader) (*InfomaniakAPIResponse, error) {
	return ik.request("PUT", path, body)
}

// get is a helper to build a bare DELETE request
func (ik *InfomaniakAPI) delete(path string) (*InfomaniakAPIResponse, error) {
	return ik.request("DELETE", path, nil)
}

// InfomaniakDNSRecord defines the format of a DNSRecord object
type InfomaniakDNSRecord struct {
	ID        uint64 `json:"id,omitempty"`
	Source    string `json:"source,omitempty"`
	SourceIdn string `json:"source_idn,omitempty"`
	Type      string `json:"type,omitempty"`
	TTL       uint64 `json:"ttl,omitempty"`
	Target    string `json:"target,omitempty"`
	UpdatedAt uint64 `json:"updated_at,omitempty"`
}

// getRecordID gather a record id from its specs (zone, source, target, rtype)
func (ik *InfomaniakAPI) getRecordID(zone, source, target, rtype string) (*uint64, error) {
	klog.V(4).Infof("Getting all records for zone=%s, then match source=%s target=%s rtype=%s", zone, source, target, rtype)

	resp, err := ik.get(fmt.Sprintf("/2/zones/%s/records", url.PathEscape(zone)), nil)
	if err != nil {
		return nil, err
	}

	if resp.Data == nil {
		return nil, fmt.Errorf("no data in response")
	}

	var records []InfomaniakDNSRecord

	if err = json.Unmarshal(*resp.Data, &records); err != nil {
		return nil, fmt.Errorf("expected array of Record, got: %v", string(*resp.Data))
	}

	for _, record := range records {
		if record.Source == source && record.Type == rtype && unquoteTarget(record.Target) == target {
			return &record.ID, nil
		}
	}

	return nil, nil
}

// unquoteTarget unquotes a dns record target when it is written in the quoted
// zone file format (TXT records are returned quoted by the API)
func unquoteTarget(target string) string {
	unquoted, err := strconv.Unquote(target)
	if err != nil {
		return target
	}
	return unquoted
}

// EnsureDNSRecord ensures a record is present with the correct key
func (ik *InfomaniakAPI) EnsureDNSRecord(zone, source, target, rtype string, ttl uint64) error {
	klog.V(4).Infof("Ensure record zone=%s source=%s target=%s rtype=%s TTL=%d", zone, source, target, rtype, ttl)

	recordID, err := ik.getRecordID(zone, source, target, rtype)
	if err != nil {
		return err
	}

	if recordID != nil {
		klog.V(4).Infof("Record already exists (zone=%s record=%d source=%s rtype=%s target=%s), skipping addition", zone, *recordID, source, rtype, target)
		return nil
	}

	record := InfomaniakDNSRecord{Source: source, Target: target, Type: rtype, TTL: ttl}
	rawJSON, err := json.Marshal(record)
	if err != nil {
		return err
	}

	klog.V(4).Infof("Adding record zone=%s (source=%s rtype=%s target=%s ttl=%d)", zone, source, rtype, target, ttl)
	_, err = ik.post(fmt.Sprintf("/2/zones/%s/records", url.PathEscape(zone)), bytes.NewBuffer(rawJSON))
	return err
}

// RemoveDNSRecord ensures a record is absent
func (ik *InfomaniakAPI) RemoveDNSRecord(zone, source, target, rtype string) error {
	klog.V(4).Infof("Remove record zone=%s source=%s rtype=%s target=%s", zone, source, rtype, target)
	recordID, err := ik.getRecordID(zone, source, target, rtype)
	if err != nil {
		return err
	}

	// the record is already absent doing nothing
	if recordID == nil {
		klog.V(4).Infof("No record found (zone=%s source=%s rtype=%s target=%s), skipping deletion", zone, source, rtype, target)
		return nil
	}

	klog.V(4).Infof("Deleting record zone=%s record=%d (source=%s rtype=%s target=%s)", zone, *recordID, source, rtype, target)
	_, err = ik.delete(fmt.Sprintf("/2/zones/%s/records/%d", url.PathEscape(zone), *recordID))
	return err
}

// UpdateDNSRecord updates the target and the TTL of an existing record
func (ik *InfomaniakAPI) UpdateDNSRecord(zone string, recordID uint64, target string, ttl uint64) error {
	klog.V(4).Infof("Update record zone=%s record=%d target=%s ttl=%d", zone, recordID, target, ttl)

	record := InfomaniakDNSRecord{Target: target, TTL: ttl}
	rawJSON, err := json.Marshal(record)
	if err != nil {
		return err
	}

	_, err = ik.put(fmt.Sprintf("/2/zones/%s/records/%d", url.PathEscape(zone), recordID), bytes.NewBuffer(rawJSON))
	return err
}
