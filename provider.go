// Package libdnstemplate implements a DNS record management client compatible
// with the libdns interfaces for <PROVIDER NAME>. TODO: This package is a
// template only. Customize all godocs for actual implementation.
package namedrop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/libdns/libdns"
)

type NamedropRequest struct {
	Domain  string            `json:"domain,omitempty"`
	Host    string            `json:"host,omitempty"`
	Token   string            `json:"token,omitempty"`
	Records []*NamedropRecord `json:"records,omitempty"`
}

type NamedropResponse struct {
	Type    string            `json:"type,omitempty"`
	Records []*NamedropRecord `json:"records,omitempty"`
}

type NamedropRecord struct {
	Domain   string `json:"domain,omitempty"`
	Host     string `json:"host,omitempty"`
	Type     string `json:"type,omitempty"`
	Value    string `json:"value,omitempty"`
	Ttl      int    `json:"ttl,omitempty"`
	Priority int    `json:"priority,omitempty"`
}

// Provider facilitates DNS record manipulation with NameDrop.
type Provider struct {
	ServerUri  string `json:"server_uri,omitempty"`
	Token      string `json:"token,omitempty"`
	httpClient *http.Client
}

// GetRecords lists all the records in the zone.
func (p *Provider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {

	ndReq := &NamedropRequest{
		Domain: zoneToDomain(zone),
		Token:  p.Token,
	}

	ndRes, err := p.namedropRequest("/get-records", ndReq)
	if err != nil {
		return nil, err
	}

	records := namedropRecordsToLibdnsRecords(ndRes.Records)

	return records, nil
}

// AppendRecords adds records to the zone. It returns the records that were added.
func (p *Provider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	return p.mutateRequest(zoneToDomain(zone), "/create-records", records)
}

// SetRecords sets the records in the zone, either by updating existing records or creating new ones.
// It returns the updated records.
func (p *Provider) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	return p.mutateRequest(zoneToDomain(zone), "/set-records", records)
}

// DeleteRecords deletes the records from the zone. It returns the records that were deleted.
func (p *Provider) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	return p.mutateRequest(zoneToDomain(zone), "/delete-records", records)
}

func (p *Provider) getServerUri() string {
	if p.ServerUri == "" {
		p.ServerUri = "https://takingnames.io/namedrop"
	}
	return p.ServerUri
}

func (p *Provider) getClient() *http.Client {
	if p.httpClient == nil {
		p.httpClient = &http.Client{}
	}
	return p.httpClient
}

func (p *Provider) mutateRequest(zone, endpoint string, records []libdns.Record) ([]libdns.Record, error) {
	ndRecs := libdnsRecordsToNamedropRecords(records)

	ndReq := &NamedropRequest{
		Domain:  zoneToDomain(zone),
		Token:   p.Token,
		Records: ndRecs,
	}

	ndRes, err := p.namedropRequest(endpoint, ndReq)
	if err != nil {
		return nil, err
	}

	return namedropRecordsToLibdnsRecords(ndRes.Records), nil
}

func (p *Provider) namedropRequest(endpoint string, req *NamedropRequest) (*NamedropResponse, error) {

	client := p.getClient()

	uri := fmt.Sprintf("%s%s", p.getServerUri(), endpoint)

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	res, err := client.Post(uri, "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("Bad status code %d: %s\n", res.StatusCode, string(bodyBytes))
	}

	var ndRes *NamedropResponse

	err = json.Unmarshal(bodyBytes, &ndRes)
	if err != nil {
		return nil, err
	}

	return ndRes, nil
}

func libdnsRecordsToNamedropRecords(records []libdns.Record) []*NamedropRecord {

	ndRecs := []*NamedropRecord{}
	for _, rec := range records {
		ndRec := &NamedropRecord{
			Host:  rec.Name,
			Type:  rec.Type,
			Value: rec.Value,
			//Ttl: int(rec.TTL),
			Priority: rec.Priority,
		}
		ndRecs = append(ndRecs, ndRec)
	}

	return ndRecs
}

func namedropRecordsToLibdnsRecords(ndRecs []*NamedropRecord) []libdns.Record {
	records := []libdns.Record{}

	for _, ndRec := range ndRecs {
		record := libdns.Record{
			Name:     ndRec.Host,
			Type:     ndRec.Type,
			Value:    ndRec.Value,
			TTL:      time.Second * time.Duration(ndRec.Ttl),
			Priority: ndRec.Priority,
		}
		records = append(records, record)
	}

	return records
}

func zoneToDomain(zone string) string {
	if strings.HasSuffix(zone, ".") {
		return zone[:len(zone)-1]
	}
	return zone
}

func printJson(data interface{}) {
	d, _ := json.MarshalIndent(data, "", "  ")
	fmt.Fprintln(os.Stderr, string(d))
}

// Interface guards
var (
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
)
