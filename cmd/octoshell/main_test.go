package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"octoshell/internal/octopus"
)

func TestRunOutputsCSVWithDirectToken(t *testing.T) {
	t.Helper()

	octopusNow := octopusNowFunc(t, time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC))
	defer octopusNow()

	newClient = func(endpoint string, _ *http.Client) *octopus.Client {
		return octopus.NewClient(endpoint, &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				var req map[string]any
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatalf("decode request: %v", err)
				}

				query := req["query"].(string)
				variables := req["variables"].(map[string]any)

				switch {
				case strings.Contains(query, "query AccountPropertyMetadata"):
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
								},
							},
						},
					}), nil
				case strings.Contains(query, "query AccountHalfHourlyReadings"):
					if variables["fromDatetime"] == "2025-01-01T00:00:00+09:00" {
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
									},
								},
							},
						}), nil
					}
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
								},
							},
						},
					}), nil
				default:
					t.Fatalf("unexpected query: %s", query)
				}
				return nil, io.EOF
			}),
		})
	}
	defer func() { newClient = octopus.NewClient }()

	var stdout bytes.Buffer
	err := run(context.Background(), &stdout, &bytes.Buffer{}, []string{
		"--account-number", "A-123",
		"--token", "token-123",
		"--format", "csv",
		"--endpoint", "https://example.test/graphql",
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "property_id,month,start_at,end_at,read_at,value,unit,type_name") {
		t.Fatalf("missing csv header: %s", got)
	}
	if !strings.Contains(got, "prop-1,2025-01,2025-01-01T00:00:00+09:00,2025-02-01T00:00:00+09:00,2025-02-01T00:00:00+09:00,0.5,,ElectricityHalfHourReadingAggregate") {
		t.Fatalf("missing csv row: %s", got)
	}
}

func TestRunObtainsTokenWhenMissing(t *testing.T) {
	t.Helper()

	octopusNow := octopusNowFunc(t, time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC))
	defer octopusNow()

	var authHeaders []string
	newClient = func(endpoint string, _ *http.Client) *octopus.Client {
		return octopus.NewClient(endpoint, &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				authHeaders = append(authHeaders, r.Header.Get("Authorization"))

				var req map[string]any
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatalf("decode request: %v", err)
				}

				query := req["query"].(string)
				switch {
				case strings.Contains(query, "mutation ObtainKrakenToken"):
					return jsonResponse(t, map[string]any{
						"data": map[string]any{
							"obtainKrakenToken": map[string]any{
								"token": "issued-token",
							},
						},
					}), nil
				case strings.Contains(query, "query AccountPropertyMetadata"):
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
				case strings.Contains(query, "query AccountHalfHourlyReadings"):
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
								},
							},
						},
					}), nil
				default:
					t.Fatalf("unexpected query: %s", query)
				}
				return nil, io.EOF
			}),
		})
	}
	defer func() { newClient = octopus.NewClient }()

	err := run(context.Background(), &bytes.Buffer{}, &bytes.Buffer{}, []string{
		"--account-number", "A-123",
		"--email", "user@example.com",
		"--password", "secret",
		"--endpoint", "https://example.test/graphql",
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if len(authHeaders) < 3 {
		t.Fatalf("unexpected request count: %d", len(authHeaders))
	}
	if authHeaders[0] != "" {
		t.Fatalf("expected empty auth header for token mutation, got %q", authHeaders[0])
	}
	if authHeaders[1] != "issued-token" || authHeaders[2] != "issued-token" {
		t.Fatalf("expected issued-token on authenticated requests, got %#v", authHeaders)
	}
}

func TestParseFlagsRequiresAuthentication(t *testing.T) {
	_, err := parseFlags(&bytes.Buffer{}, []string{
		"--account-number", "A-123",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "token or email/password") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseFlagsTrimsWhitespaceToken(t *testing.T) {
	_, err := parseFlags(&bytes.Buffer{}, []string{
		"--account-number", " A-123 ",
		"--token", "   ",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "token or email/password") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseFlagsRejectsInvalidFormat(t *testing.T) {
	_, err := parseFlags(&bytes.Buffer{}, []string{
		"--account-number", "A-123",
		"--token", "token-123",
		"--format", "xml",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "format must be json or csv") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunOutputsVersion(t *testing.T) {
	previous := version
	version = "0.1.0-test"
	defer func() { version = previous }()

	var stdout bytes.Buffer
	err := run(context.Background(), &stdout, &bytes.Buffer{}, []string{"--version"})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if strings.TrimSpace(stdout.String()) != "0.1.0-test" {
		t.Fatalf("unexpected version output: %q", stdout.String())
	}
}

func TestParseFlagsPreservesPasswordWhitespace(t *testing.T) {
	cfg, err := parseFlags(&bytes.Buffer{}, []string{
		"--account-number", "A-123",
		"--email", "user@example.com",
		"--password", " secret ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.password != " secret " {
		t.Fatalf("password was modified: %q", cfg.password)
	}
}

func TestRunVerboseOutputsDebugDetails(t *testing.T) {
	t.Helper()

	octopusNow := octopusNowFunc(t, time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC))
	defer octopusNow()

	newClient = func(endpoint string, _ *http.Client) *octopus.Client {
		return octopus.NewClient(endpoint, &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				var req map[string]any
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatalf("decode request: %v", err)
				}

				query := req["query"].(string)
				switch {
				case strings.Contains(query, "mutation ObtainKrakenToken"):
					return jsonResponse(t, map[string]any{
						"data": map[string]any{
							"obtainKrakenToken": map[string]any{
								"token": "issued-token",
							},
						},
					}), nil
				case strings.Contains(query, "query AccountPropertyMetadata"):
					return jsonResponse(t, map[string]any{
						"errors": []map[string]any{
							{"message": "Invalid data."},
						},
					}), nil
				default:
					t.Fatalf("unexpected query: %s", query)
				}
				return nil, io.EOF
			}),
		})
	}
	defer func() { newClient = octopus.NewClient }()

	var stderr bytes.Buffer
	err := run(context.Background(), &bytes.Buffer{}, &stderr, []string{
		"-v",
		"--account-number", "A-123",
		"--email", "user@example.com",
		"--password", "secret",
		"--endpoint", "https://example.test/graphql",
	})
	if err == nil {
		t.Fatal("expected error")
	}

	logs := stderr.String()
	if !strings.Contains(logs, "[graphql] request query:") {
		t.Fatalf("missing debug query log: %s", logs)
	}
	if !strings.Contains(logs, "AccountPropertyMetadata") {
		t.Fatalf("missing metadata query log: %s", logs)
	}
	if !strings.Contains(logs, `"password": "***"`) {
		t.Fatalf("expected redacted password in logs: %s", logs)
	}
	if strings.Contains(logs, "secret") {
		t.Fatalf("password leaked in logs: %s", logs)
	}
	if !strings.Contains(logs, "Invalid data.") {
		t.Fatalf("missing graphql error body: %s", logs)
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
		t.Fatalf("encode response: %v", err)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(strings.NewReader(string(body))),
	}
}

func octopusNowFunc(t *testing.T, now time.Time) func() {
	t.Helper()

	previous := octopus.SetNowFuncForTest(func() time.Time {
		return now
	})
	return func() {
		octopus.SetNowFuncForTest(previous)
	}
}
