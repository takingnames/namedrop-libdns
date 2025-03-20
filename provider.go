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
	"github.com/takingnames/namedrop-go"
)

type NamedropResponse struct {
	Type    string             `json:"type,omitempty"`
	Records []*namedrop.Record `json:"records,omitempty"`
}

// Provider facilitates DNS record manipulation with NameDrop.
type Provider struct {
	ServerUri  string                  `json:"server_uri,omitempty"`
	TokenData  *namedrop.TokenResponse `json:"token_data,omitempty"`
	httpClient *http.Client
}

// GetRecords lists all the records in the zone.
func (p *Provider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {

	ndReq := &namedrop.RecordsRequest{
		Domain: zoneToDomain(zone),
		Token:  p.TokenData.AccessToken,
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
	// TODO: might need to implement append-records for NameDrop
	//return p.mutateRequest(zoneToDomain(zone), "/create-records", records)
	return p.mutateRequest(zoneToDomain(zone), "/set-records", records)
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

func (p *Provider) ListZones(ctx context.Context) ([]libdns.Zone, error) {
	zones := []libdns.Zone{}

	for _, perm := range p.TokenData.Permissions {
		domain := perm.Domain
		if perm.Host != "" {
			domain = perm.Host + "." + perm.Domain
		}
		zone := libdns.Zone{
			Name: domain,
		}
		zones = append(zones, zone)
	}

	return zones, nil
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

	ndReq := &namedrop.RecordsRequest{
		Domain:  zoneToDomain(zone),
		Token:   p.TokenData.AccessToken,
		Records: ndRecs,
	}

	ndRes, err := p.namedropRequest(endpoint, ndReq)
	if err != nil {
		return nil, err
	}

	fmt.Println(ndRes)
	printJson(ndRes)

	// TODO: might need to return actual created records for NameDrop
	return records, nil
	//return namedropRecordsToLibdnsRecords(ndRes.Records), nil
}

func (p *Provider) namedropRequest(endpoint string, req *namedrop.RecordsRequest) (*NamedropResponse, error) {

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

func libdnsRecordsToNamedropRecords(records []libdns.Record) []*namedrop.Record {

	ndRecs := []*namedrop.Record{}
	for _, rec := range records {
		ndRec := &namedrop.Record{
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

func namedropRecordsToLibdnsRecords(ndRecs []*namedrop.Record) []libdns.Record {
	records := []libdns.Record{}

	for _, ndRec := range ndRecs {
		record := libdns.Record{
			Name:     ndRec.Host,
			Type:     ndRec.Type,
			Value:    ndRec.Value,
			TTL:      time.Second * time.Duration(ndRec.TTL),
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
	_ libdns.ZoneLister     = (*Provider)(nil)
)
