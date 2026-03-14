package octopus

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestMonthlyElectricityUsageAggregatesHalfHourlyReadingsPerMonth(t *testing.T) {
	t.Helper()

	restoreNow := SetNowFuncForTest(func() time.Time {
		return time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	})
	defer SetNowFuncForTest(restoreNow)

	var monthQueries []string
	client := NewClient("https://example.test/graphql", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.Header.Get("Authorization"); got != "token-123" {
				t.Fatalf("unexpected authorization header: %s", got)
			}

			var req graphqlRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}

			switch {
			case strings.Contains(req.Query, "query AccountPropertyMetadata"):
				return jsonResponse(t, map[string]any{
					"data": map[string]any{
						"account": map[string]any{
							"properties": []map[string]any{
								{
									"id": "prop-1",
									"electricitySupplyPoints": []map[string]any{
										{
											"agreements": []map[string]any{
												{"validFrom": "2025-01-10T00:00:00+09:00"},
											},
										},
									},
								},
								{
									"id": "prop-2",
									"electricitySupplyPoints": []map[string]any{
										{
											"agreements": []map[string]any{
												{"validFrom": "2025-02-01T00:00:00+09:00"},
											},
										},
									},
								},
							},
						},
					},
				}), nil
			case strings.Contains(req.Query, "query AccountHalfHourlyReadings"):
				vars := req.Variables.(map[string]any)
				from := vars["fromDatetime"].(string)
				monthQueries = append(monthQueries, from)

				switch from {
				case "2025-01-01T00:00:00+09:00":
					return jsonResponse(t, map[string]any{
						"data": map[string]any{
							"account": map[string]any{
								"properties": []map[string]any{
									{
										"id": "prop-1",
										"electricitySupplyPoints": []map[string]any{
											{
												"halfHourlyReadings": []map[string]any{
													{
														"startAt": "2025-01-10T00:00:00+09:00",
														"endAt":   "2025-01-10T00:30:00+09:00",
														"value":   "0.20",
													},
													{
														"startAt": "2025-01-10T00:30:00+09:00",
														"endAt":   "2025-01-10T01:00:00+09:00",
														"value":   "0.30",
													},
												},
											},
										},
									},
									{
										"id": "prop-2",
										"electricitySupplyPoints": []map[string]any{
											{
												"halfHourlyReadings": []map[string]any{},
											},
										},
									},
								},
							},
						},
					}), nil
				case "2025-02-01T00:00:00+09:00":
					return jsonResponse(t, map[string]any{
						"data": map[string]any{
							"account": map[string]any{
								"properties": []map[string]any{
									{
										"id": "prop-1",
										"electricitySupplyPoints": []map[string]any{
											{
												"halfHourlyReadings": []map[string]any{
													{
														"startAt": "2025-02-01T00:00:00+09:00",
														"endAt":   "2025-02-01T00:30:00+09:00",
														"value":   "1.25",
													},
												},
											},
										},
									},
									{
										"id": "prop-2",
										"electricitySupplyPoints": []map[string]any{
											{
												"halfHourlyReadings": []map[string]any{
													{
														"startAt": "2025-02-01T00:00:00+09:00",
														"endAt":   "2025-02-01T00:30:00+09:00",
														"value":   "0.75",
													},
												},
											},
										},
									},
								},
							},
						},
					}), nil
				case "2025-03-01T00:00:00+09:00":
					return jsonResponse(t, map[string]any{
						"data": map[string]any{
							"account": map[string]any{
								"properties": []map[string]any{
									{
										"id": "prop-1",
										"electricitySupplyPoints": []map[string]any{
											{
												"halfHourlyReadings": []map[string]any{},
											},
										},
									},
									{
										"id": "prop-2",
										"electricitySupplyPoints": []map[string]any{
											{
												"halfHourlyReadings": []map[string]any{
													{
														"startAt": "2025-03-01T00:00:00+09:00",
														"endAt":   "2025-03-01T00:30:00+09:00",
														"value":   "0.50",
													},
												},
											},
										},
									},
								},
							},
						},
					}), nil
				default:
					t.Fatalf("unexpected month window: %s", from)
				}
			default:
				t.Fatalf("unexpected query: %s", req.Query)
			}
			return nil, errors.New("unreachable")
		}),
	})

	got, err := client.MonthlyElectricityUsage(context.Background(), "token-123", UsageParams{
		AccountNumber: "A-123",
		Timezone:      "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("MonthlyElectricityUsage returned error: %v", err)
	}

	if len(monthQueries) != 3 {
		t.Fatalf("unexpected month query count: %d", len(monthQueries))
	}
	if len(got) != 4 {
		t.Fatalf("unexpected usage count: %d", len(got))
	}
	if got[0].PropertyID != "prop-1" || got[0].Month != "2025-01" || got[0].Value != "0.5" {
		t.Fatalf("unexpected first row: %+v", got[0])
	}
	if got[1].PropertyID != "prop-1" || got[1].Month != "2025-02" || got[1].Value != "1.25" {
		t.Fatalf("unexpected second row: %+v", got[1])
	}
	if got[2].PropertyID != "prop-2" || got[2].Month != "2025-02" || got[2].Value != "0.75" {
		t.Fatalf("unexpected third row: %+v", got[2])
	}
	if got[3].PropertyID != "prop-2" || got[3].Month != "2025-03" || got[3].Value != "0.5" {
		t.Fatalf("unexpected fourth row: %+v", got[3])
	}
}

func TestMonthlyElectricityUsageFiltersProperty(t *testing.T) {
	t.Helper()

	client := NewClient("https://example.test/graphql", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(t, map[string]any{
				"data": map[string]any{
					"account": map[string]any{
						"properties": []map[string]any{
							{
								"id": "prop-1",
								"electricitySupplyPoints": []map[string]any{
									{
										"agreements": []map[string]any{
											{"validFrom": "2025-01-01T00:00:00+09:00"},
										},
									},
								},
							},
						},
					},
				},
			}), nil
		}),
	})

	_, err := client.MonthlyElectricityUsage(context.Background(), "token-123", UsageParams{
		AccountNumber: "A-123",
		PropertyID:    "missing",
	})
	if !errors.Is(err, ErrPropertyNotFound) {
		t.Fatalf("expected ErrPropertyNotFound, got %v", err)
	}
}

func TestObtainToken(t *testing.T) {
	t.Helper()

	client := NewClient("https://example.test/graphql", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			var req graphqlRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if !strings.Contains(req.Query, "mutation ObtainKrakenToken") {
				t.Fatalf("unexpected query: %s", req.Query)
			}
			return jsonResponse(t, map[string]any{
				"data": map[string]any{
					"obtainKrakenToken": map[string]any{
						"token": "issued-token",
					},
				},
			}), nil
		}),
	})

	token, err := client.ObtainToken(context.Background(), "user@example.com", "secret")
	if err != nil {
		t.Fatalf("ObtainToken returned error: %v", err)
	}
	if token != "issued-token" {
		t.Fatalf("unexpected token: %s", token)
	}
}

func TestObtainTokenRejectsEmptyToken(t *testing.T) {
	t.Helper()

	client := NewClient("https://example.test/graphql", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(t, map[string]any{
				"data": map[string]any{
					"obtainKrakenToken": map[string]any{
						"token": "   ",
					},
				},
			}), nil
		}),
	})

	_, err := client.ObtainToken(context.Background(), "user@example.com", "secret")
	if !errors.Is(err, ErrTokenResponseEmpty) {
		t.Fatalf("expected ErrTokenResponseEmpty, got %v", err)
	}
}

func TestMonthlyElectricityUsageReturnsGraphQLError(t *testing.T) {
	t.Helper()

	client := NewClient("https://example.test/graphql", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(t, map[string]any{
				"errors": []map[string]any{
					{"message": "permission denied"},
				},
			}), nil
		}),
	})

	_, err := client.MonthlyElectricityUsage(context.Background(), "token-123", UsageParams{
		AccountNumber: "A-123",
	})
	if !errors.Is(err, ErrGraphQLRequest) {
		t.Fatalf("expected ErrGraphQLRequest, got %v", err)
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMonthlyElectricityUsageReturnsHTTPError(t *testing.T) {
	t.Helper()

	client := NewClient("https://example.test/graphql", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{"detail":"upstream failed"}`)),
			}, nil
		}),
	})

	_, err := client.MonthlyElectricityUsage(context.Background(), "token-123", UsageParams{
		AccountNumber: "A-123",
	})
	if !errors.Is(err, ErrGraphQLRequest) {
		t.Fatalf("expected ErrGraphQLRequest, got %v", err)
	}
	if !strings.Contains(err.Error(), "status=502") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMonthlyElectricityUsageRejectsInvalidTimezone(t *testing.T) {
	_, err := NewClient("https://example.test/graphql", &http.Client{}).MonthlyElectricityUsage(context.Background(), "token-123", UsageParams{
		AccountNumber: "A-123",
		Timezone:      "Mars/Phobos",
	})
	if !errors.Is(err, ErrInvalidTimezone) {
		t.Fatalf("expected ErrInvalidTimezone, got %v", err)
	}
}

func TestWriteCSV(t *testing.T) {
	var builder strings.Builder
	err := WriteCSV(&builder, []MonthlyUsage{
		{
			PropertyID: "prop-1",
			Month:      "2025-01",
			StartAt:    "2025-01-01T00:00:00+09:00",
			EndAt:      "2025-02-01T00:00:00+09:00",
			ReadAt:     "2025-02-01T00:00:00+09:00",
			Value:      "0.5",
			TypeName:   "ElectricityHalfHourReadingAggregate",
		},
	})
	if err != nil {
		t.Fatalf("WriteCSV returned error: %v", err)
	}

	got := builder.String()
	if !strings.Contains(got, "property_id,month,start_at,end_at,read_at,value,unit,type_name") {
		t.Fatalf("missing csv header: %s", got)
	}
	if !strings.Contains(got, "prop-1,2025-01,2025-01-01T00:00:00+09:00,2025-02-01T00:00:00+09:00,2025-02-01T00:00:00+09:00,0.5,,ElectricityHalfHourReadingAggregate") {
		t.Fatalf("missing csv row: %s", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(t *testing.T, payload map[string]any) *http.Response {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode json: %v", err)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(strings.NewReader(string(body))),
	}
}
