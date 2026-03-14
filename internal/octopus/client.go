package octopus

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"time"
)

const DefaultEndpoint = "https://api.oejp-kraken.energy/v1/graphql/"

var (
	ErrMissingAuth        = errors.New("either token or email/password must be provided")
	ErrMissingAccount     = errors.New("account number is required")
	ErrInvalidTimezone    = errors.New("timezone is invalid")
	ErrPropertyNotFound   = errors.New("property id not found on account")
	ErrGraphQLRequest     = errors.New("graphql request failed")
	ErrTokenResponseEmpty = errors.New("token response was empty")
	nowFunc               = time.Now
)

type Client struct {
	endpoint    string
	httpClient  *http.Client
	debugWriter io.Writer
}

type UsageParams struct {
	AccountNumber string
	PropertyID    string
	Timezone      string
}

type MonthlyUsage struct {
	PropertyID string `json:"propertyId"`
	Month      string `json:"month"`
	StartAt    string `json:"startAt,omitempty"`
	EndAt      string `json:"endAt,omitempty"`
	ReadAt     string `json:"readAt,omitempty"`
	Value      string `json:"value"`
	Unit       string `json:"unit,omitempty"`
	TypeName   string `json:"typeName"`
}

type graphqlRequest struct {
	Query     string      `json:"query"`
	Variables interface{} `json:"variables,omitempty"`
}

type graphqlResponse[T any] struct {
	Data   T              `json:"data"`
	Errors []graphqlError `json:"errors"`
}

type graphqlError struct {
	Message string `json:"message"`
}

type tokenResponse struct {
	ObtainKrakenToken struct {
		Token string `json:"token"`
	} `json:"obtainKrakenToken"`
}

type accountMetadataResponse struct {
	Account struct {
		Properties []accountPropertyMetadata `json:"properties"`
	} `json:"account"`
}

type accountPropertyMetadata struct {
	ID                      string                           `json:"id"`
	ElectricitySupplyPoints []electricitySupplyPointMetadata `json:"electricitySupplyPoints"`
}

type electricitySupplyPointMetadata struct {
	Agreements []agreement `json:"agreements"`
}

type agreement struct {
	ValidFrom string `json:"validFrom"`
}

type accountHalfHourlyReadingsResponse struct {
	Account struct {
		Properties []accountPropertyReadings `json:"properties"`
	} `json:"account"`
}

type accountPropertyReadings struct {
	ID                      string                           `json:"id"`
	ElectricitySupplyPoints []electricitySupplyPointReadings `json:"electricitySupplyPoints"`
}

type electricitySupplyPointReadings struct {
	HalfHourlyReadings []halfHourlyReading `json:"halfHourlyReadings"`
}

type halfHourlyReading struct {
	StartAt string       `json:"startAt"`
	EndAt   string       `json:"endAt"`
	Value   numberString `json:"value"`
}

type numberString string

func (n *numberString) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" {
		*n = ""
		return nil
	}

	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		*n = numberString(asString)
		return nil
	}

	*n = numberString(trimmed)
	return nil
}

func NewClient(endpoint string, httpClient *http.Client) *Client {
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		endpoint:   endpoint,
		httpClient: httpClient,
	}
}

func (c *Client) SetDebugWriter(w io.Writer) {
	c.debugWriter = w
}

func (c *Client) ObtainToken(ctx context.Context, email, password string) (string, error) {
	const query = `
mutation ObtainKrakenToken($input: ObtainJSONWebTokenInput!) {
  obtainKrakenToken(input: $input) {
    token
  }
}`

	variables := map[string]interface{}{
		"input": map[string]string{
			"email":    email,
			"password": password,
		},
	}

	resp, err := doGraphQL[tokenResponse](ctx, c, "", query, variables)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(resp.ObtainKrakenToken.Token) == "" {
		return "", ErrTokenResponseEmpty
	}
	return resp.ObtainKrakenToken.Token, nil
}

func (c *Client) MonthlyElectricityUsage(ctx context.Context, token string, params UsageParams) ([]MonthlyUsage, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrMissingAuth
	}
	if strings.TrimSpace(params.AccountNumber) == "" {
		return nil, ErrMissingAccount
	}

	timezone := strings.TrimSpace(params.Timezone)
	if timezone == "" {
		timezone = "Asia/Tokyo"
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidTimezone, timezone)
	}

	propertyStarts, err := c.propertyStartDates(ctx, token, params.AccountNumber)
	if err != nil {
		return nil, err
	}

	selectedIDs, err := filterPropertyIDs(propertyStarts, strings.TrimSpace(params.PropertyID))
	if err != nil {
		return nil, err
	}
	if len(selectedIDs) == 0 {
		return nil, nil
	}

	firstMonth := earliestMonthStart(selectedIDs, propertyStarts, loc)
	if firstMonth.IsZero() {
		return nil, nil
	}

	now := nowFunc().In(loc)
	var all []MonthlyUsage
	for windowStart := firstMonth; windowStart.Before(now); windowStart = windowStart.AddDate(0, 1, 0) {
		windowEnd := windowStart.AddDate(0, 1, 0)
		if windowEnd.After(now) {
			windowEnd = now
		}

		usages, err := c.monthlyUsageWindow(ctx, token, params.AccountNumber, selectedIDs, windowStart, windowEnd)
		if err != nil {
			return nil, err
		}
		all = append(all, usages...)
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].PropertyID != all[j].PropertyID {
			return all[i].PropertyID < all[j].PropertyID
		}
		return all[i].Month < all[j].Month
	})

	return all, nil
}

func WriteCSV(w io.Writer, usages []MonthlyUsage) error {
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"property_id", "month", "start_at", "end_at", "read_at", "value", "unit", "type_name"}); err != nil {
		return err
	}
	for _, usage := range usages {
		record := []string{
			usage.PropertyID,
			usage.Month,
			usage.StartAt,
			usage.EndAt,
			usage.ReadAt,
			usage.Value,
			usage.Unit,
			usage.TypeName,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func (c *Client) propertyStartDates(ctx context.Context, token, accountNumber string) (map[string]time.Time, error) {
	const query = `
query AccountPropertyMetadata($accountNumber: String!) {
  account(accountNumber: $accountNumber) {
    properties {
      id
      electricitySupplyPoints {
        agreements {
          validFrom
        }
      }
    }
  }
}`

	resp, err := doGraphQL[accountMetadataResponse](ctx, c, token, query, map[string]string{
		"accountNumber": accountNumber,
	})
	if err != nil {
		return nil, err
	}

	propertyStarts := make(map[string]time.Time, len(resp.Account.Properties))
	for _, property := range resp.Account.Properties {
		if property.ID == "" {
			continue
		}
		var earliest time.Time
		for _, supplyPoint := range property.ElectricitySupplyPoints {
			for _, item := range supplyPoint.Agreements {
				if strings.TrimSpace(item.ValidFrom) == "" {
					continue
				}
				validFrom, err := parseTime(item.ValidFrom)
				if err != nil {
					continue
				}
				if earliest.IsZero() || validFrom.Before(earliest) {
					earliest = validFrom
				}
			}
		}
		propertyStarts[property.ID] = earliest
	}
	return propertyStarts, nil
}

func (c *Client) monthlyUsageWindow(ctx context.Context, token, accountNumber string, selectedIDs []string, windowStart, windowEnd time.Time) ([]MonthlyUsage, error) {
	const query = `
query AccountHalfHourlyReadings($accountNumber: String!, $fromDatetime: DateTime, $toDatetime: DateTime) {
  account(accountNumber: $accountNumber) {
    properties {
      id
      electricitySupplyPoints {
        halfHourlyReadings(fromDatetime: $fromDatetime, toDatetime: $toDatetime) {
          startAt
          endAt
          value
        }
      }
    }
  }
}`

	resp, err := doGraphQL[accountHalfHourlyReadingsResponse](ctx, c, token, query, map[string]string{
		"accountNumber": accountNumber,
		"fromDatetime":  windowStart.Format(time.RFC3339),
		"toDatetime":    windowEnd.Format(time.RFC3339),
	})
	if err != nil {
		return nil, err
	}

	selected := make(map[string]struct{}, len(selectedIDs))
	for _, id := range selectedIDs {
		selected[id] = struct{}{}
	}

	month := windowStart.Format("2006-01")
	startAt := windowStart.Format(time.RFC3339)
	endAt := windowEnd.Format(time.RFC3339)

	var usages []MonthlyUsage
	for _, property := range resp.Account.Properties {
		if _, ok := selected[property.ID]; !ok {
			continue
		}

		total := new(big.Rat)
		readingCount := 0
		for _, supplyPoint := range property.ElectricitySupplyPoints {
			for _, reading := range supplyPoint.HalfHourlyReadings {
				if err := addDecimal(total, string(reading.Value)); err != nil {
					return nil, fmt.Errorf("parse reading value for property %s: %w", property.ID, err)
				}
				readingCount++
			}
		}
		if readingCount == 0 {
			continue
		}

		usages = append(usages, MonthlyUsage{
			PropertyID: property.ID,
			Month:      month,
			StartAt:    startAt,
			EndAt:      endAt,
			ReadAt:     endAt,
			Value:      formatRat(total),
			TypeName:   "ElectricityHalfHourReadingAggregate",
		})
	}

	return usages, nil
}

func doGraphQL[T any](ctx context.Context, client *Client, token, query string, variables interface{}) (T, error) {
	var zero T

	payload, err := json.Marshal(graphqlRequest{
		Query:     query,
		Variables: variables,
	})
	if err != nil {
		return zero, fmt.Errorf("marshal graphql request: %w", err)
	}
	client.debugf("[graphql] request query:\n%s\n", strings.TrimSpace(query))
	if client.debugWriter != nil {
		if sanitized, err := json.MarshalIndent(sanitizeForDebug(variables), "", "  "); err == nil {
			client.debugf("[graphql] request variables:\n%s\n", string(sanitized))
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, bytes.NewReader(payload))
	if err != nil {
		return zero, fmt.Errorf("build graphql request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", token)
	}

	res, err := client.httpClient.Do(req)
	if err != nil {
		client.debugf("[graphql] request error: %v\n", err)
		return zero, fmt.Errorf("%w: %v", ErrGraphQLRequest, err)
	}
	defer res.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if readErr != nil {
		return zero, fmt.Errorf("read graphql response: %w", readErr)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		client.debugf("[graphql] response status=%d body=%s\n", res.StatusCode, strings.TrimSpace(string(body)))
		return zero, fmt.Errorf("%w: status=%d body=%s", ErrGraphQLRequest, res.StatusCode, strings.TrimSpace(string(body)))
	}

	var envelope graphqlResponse[T]
	if err := json.Unmarshal(body, &envelope); err != nil {
		client.debugf("[graphql] response decode failed body=%s\n", strings.TrimSpace(string(body)))
		return zero, fmt.Errorf("decode graphql response: %w", err)
	}

	if len(envelope.Errors) > 0 {
		client.debugf("[graphql] response graphql-errors body=%s\n", strings.TrimSpace(string(body)))
		messages := make([]string, 0, len(envelope.Errors))
		for _, gqlErr := range envelope.Errors {
			if gqlErr.Message != "" {
				messages = append(messages, gqlErr.Message)
			}
		}
		return zero, fmt.Errorf("%w: %s", ErrGraphQLRequest, strings.Join(messages, "; "))
	}

	return envelope.Data, nil
}

func filterPropertyIDs(propertyStarts map[string]time.Time, propertyID string) ([]string, error) {
	if propertyID != "" {
		if _, ok := propertyStarts[propertyID]; !ok {
			return nil, ErrPropertyNotFound
		}
		return []string{propertyID}, nil
	}

	ids := make([]string, 0, len(propertyStarts))
	for id := range propertyStarts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func earliestMonthStart(propertyIDs []string, propertyStarts map[string]time.Time, loc *time.Location) time.Time {
	var earliest time.Time
	for _, propertyID := range propertyIDs {
		start := propertyStarts[propertyID]
		if start.IsZero() {
			continue
		}
		monthStart := time.Date(start.In(loc).Year(), start.In(loc).Month(), 1, 0, 0, 0, 0, loc)
		if earliest.IsZero() || monthStart.Before(earliest) {
			earliest = monthStart
		}
	}
	return earliest
}

func parseTime(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time format: %s", value)
}

func sanitizeForDebug(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		cloned := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			if isSensitiveKey(key) {
				cloned[key] = "***"
				continue
			}
			cloned[key] = sanitizeForDebug(item)
		}
		return cloned
	case map[string]string:
		cloned := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			if isSensitiveKey(key) {
				cloned[key] = "***"
				continue
			}
			cloned[key] = item
		}
		return cloned
	case []interface{}:
		cloned := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			cloned = append(cloned, sanitizeForDebug(item))
		}
		return cloned
	default:
		return value
	}
}

func isSensitiveKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "password", "token", "refreshtoken", "authorization":
		return true
	default:
		return false
	}
}

func addDecimal(total *big.Rat, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	addend, ok := new(big.Rat).SetString(trimmed)
	if !ok {
		return fmt.Errorf("invalid decimal: %s", value)
	}
	total.Add(total, addend)
	return nil
}

func formatRat(value *big.Rat) string {
	if value == nil {
		return ""
	}
	s := value.FloatString(6)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" {
		return "0"
	}
	return s
}

func (c *Client) debugf(format string, args ...interface{}) {
	if c.debugWriter == nil {
		return
	}
	fmt.Fprintf(c.debugWriter, format, args...)
}

func SetNowFuncForTest(fn func() time.Time) func() time.Time {
	previous := nowFunc
	nowFunc = fn
	return previous
}
