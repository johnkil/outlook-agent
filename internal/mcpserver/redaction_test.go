package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/johnkil/outlook-agent/internal/transport"
)

func TestMailSearchHandlerRedactsPrivateMessageData(t *testing.T) {
	handler := mailSearchHandler(NewRuntime(leakyTransport{}))

	_, output, err := handler(context.Background(), nil, MailSearchInput{Query: "secret"})
	if err != nil {
		t.Fatalf("mail search handler: %v", err)
	}
	if len(output.Messages) != 1 {
		t.Fatalf("expected one message, got %d", len(output.Messages))
	}

	message := output.Messages[0].(map[string]any)
	if message["subject"] != "Safe subject" {
		t.Fatalf("expected subject preserved, got %#v", message["subject"])
	}
	if message["body"] != "[REDACTED]" {
		t.Fatalf("expected body redacted, got %#v", message["body"])
	}
	if message["accessToken"] != "[REDACTED]" {
		t.Fatalf("expected token redacted, got %#v", message["accessToken"])
	}
}

func TestMailSearchHandlerPreservesPaginationMetadata(t *testing.T) {
	handler := mailSearchHandler(NewRuntime(paginatedSearchTransport{}))

	_, output, err := handler(context.Background(), nil, MailSearchInput{Query: "planning"})
	if err != nil {
		t.Fatalf("mail search handler: %v", err)
	}
	raw, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("marshal output: %v", err)
	}
	var encoded map[string]any
	if err := json.Unmarshal(raw, &encoded); err != nil {
		t.Fatalf("decode output: %v", err)
	}

	if encoded["returned"] != float64(1) || encoded["limit"] != float64(1) || encoded["truncated"] != true {
		t.Fatalf("expected pagination metadata, got %#v", encoded)
	}
	if encoded["next_link"] != nil {
		t.Fatalf("expected raw next_link to be hidden, got %#v", encoded["next_link"])
	}
	nextCursor, _ := encoded["next_cursor"].(string)
	if nextCursor == "" || strings.Contains(nextCursor, "graph.example.test") {
		t.Fatalf("expected opaque next_cursor, got %#v", encoded["next_cursor"])
	}
}

func TestMailSearchNextConsumesCursorAndPreservesQueryFilter(t *testing.T) {
	client := &paginatedSearchNextTransport{}
	runtime := NewRuntime(client)

	_, firstPage, err := mailSearchHandler(runtime)(context.Background(), nil, MailSearchInput{Query: "planning"})
	if err != nil {
		t.Fatalf("mail search handler: %v", err)
	}
	if firstPage.NextCursor == "" {
		t.Fatalf("expected next cursor: %#v", firstPage)
	}

	_, nextPage, err := mailSearchNextHandler(runtime)(context.Background(), nil, MailSearchNextInput{Cursor: firstPage.NextCursor})
	if err != nil {
		t.Fatalf("mail search next handler: %v", err)
	}
	if nextPage.Returned != 1 || len(nextPage.Messages) != 1 {
		t.Fatalf("expected one next-page message, got %#v", nextPage)
	}
	if client.nextLinkUsed != "https://graph.example.test/v1.0/me/messages?$skiptoken=next" {
		t.Fatalf("expected provider nextLink to stay inside transport payload, got %q", client.nextLinkUsed)
	}
	if client.queryUsed != "planning" {
		t.Fatalf("expected original query to be preserved for next page, got %q", client.queryUsed)
	}

	_, _, err = mailSearchNextHandler(runtime)(context.Background(), nil, MailSearchNextInput{Cursor: firstPage.NextCursor})
	if err == nil || !strings.Contains(err.Error(), "cursor") {
		t.Fatalf("expected consumed cursor replay to fail, got %v", err)
	}
}

func TestMailSearchNextKeepsCursorWhenProviderFails(t *testing.T) {
	client := &paginatedSearchNextTransport{failNext: true}
	runtime := NewRuntime(client)

	_, firstPage, err := mailSearchHandler(runtime)(context.Background(), nil, MailSearchInput{Query: "planning"})
	if err != nil {
		t.Fatalf("mail search handler: %v", err)
	}
	if firstPage.NextCursor == "" {
		t.Fatalf("expected next cursor: %#v", firstPage)
	}

	_, _, err = mailSearchNextHandler(runtime)(context.Background(), nil, MailSearchNextInput{Cursor: firstPage.NextCursor})
	if err == nil || !strings.Contains(err.Error(), "temporary provider failure") {
		t.Fatalf("expected transient provider failure, got %v", err)
	}

	_, nextPage, err := mailSearchNextHandler(runtime)(context.Background(), nil, MailSearchNextInput{Cursor: firstPage.NextCursor})
	if err != nil {
		t.Fatalf("expected same cursor to retry after provider failure: %v", err)
	}
	if nextPage.Returned != 1 || len(nextPage.Messages) != 1 {
		t.Fatalf("expected retry to fetch one next-page message, got %#v", nextPage)
	}
}

func TestMailSearchNextDoesNotReplaySameCursorConcurrently(t *testing.T) {
	client := newBlockingSearchNextTransport()
	runtime := NewRuntime(client)

	_, firstPage, err := mailSearchHandler(runtime)(context.Background(), nil, MailSearchInput{Query: "planning"})
	if err != nil {
		t.Fatalf("mail search handler: %v", err)
	}
	if firstPage.NextCursor == "" {
		t.Fatalf("expected next cursor: %#v", firstPage)
	}

	firstDone := make(chan error, 1)
	go func() {
		_, _, err := mailSearchNextHandler(runtime)(context.Background(), nil, MailSearchNextInput{Cursor: firstPage.NextCursor})
		firstDone <- err
	}()

	select {
	case <-client.firstSearchNextStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first search_next provider call")
	}

	secondDone := make(chan error, 1)
	go func() {
		_, _, err := mailSearchNextHandler(runtime)(context.Background(), nil, MailSearchNextInput{Cursor: firstPage.NextCursor})
		secondDone <- err
	}()

	var secondErr error
	duplicateProviderCall := false
	select {
	case <-client.secondSearchNextStarted:
		duplicateProviderCall = true
	case secondErr = <-secondDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for second search_next result")
	}

	client.release()
	if err := <-firstDone; err != nil {
		t.Fatalf("first search_next failed: %v", err)
	}
	if duplicateProviderCall {
		t.Fatal("same cursor was replayed into a second provider mail.search_next call")
	}
	if secondErr == nil || !strings.Contains(secondErr.Error(), "cursor") {
		t.Fatalf("expected concurrent cursor use to fail safely, got %v", secondErr)
	}
	if calls := client.searchNextCallCount(); calls != 1 {
		t.Fatalf("expected one provider search_next call, got %d", calls)
	}
}

func TestMailFetchBodiesRedactsPerItemErrors(t *testing.T) {
	handler := mailFetchBodiesHandler(batchBodyLeakyErrorTransport{})

	_, output, err := handler(context.Background(), nil, MailFetchBodiesInput{IDs: []string{"msg-1", "msg-2"}})
	if err != nil {
		t.Fatalf("mail fetch bodies handler: %v", err)
	}
	if output.Attempted != 2 || output.Succeeded != 1 || output.Failed != 1 {
		t.Fatalf("unexpected batch coverage: %#v", output)
	}
	if len(output.Results) != 2 || output.Results[1].OK {
		t.Fatalf("expected one failed result, got %#v", output.Results)
	}
	if strings.Contains(output.Results[1].Error, "secret") || !strings.Contains(output.Results[1].Error, "[REDACTED]") {
		t.Fatalf("expected redacted per-item error, got %q", output.Results[1].Error)
	}
}

type leakyTransport struct{}

func (leakyTransport) Name() string {
	return "leaky"
}

func (leakyTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (leakyTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{}
}

func (leakyTransport) Execute(context.Context, transport.ActionRequest) transport.ActionResponse {
	return transport.ActionResponse{
		OK: true,
		Data: map[string]any{
			"messages": []any{
				map[string]any{
					"subject":     "Safe subject",
					"body":        "private body",
					"accessToken": "private token",
				},
			},
		},
	}
}

func (leakyTransport) DryRun(context.Context, transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{}
}

type batchBodyLeakyErrorTransport struct{}

func (batchBodyLeakyErrorTransport) Name() string {
	return "leaky"
}

func (batchBodyLeakyErrorTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (batchBodyLeakyErrorTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{}
}

func (batchBodyLeakyErrorTransport) Execute(_ context.Context, request transport.ActionRequest) transport.ActionResponse {
	id, _ := request.Payload["id"].(string)
	if id == "msg-2" {
		return transport.ActionResponse{OK: false, Error: "provider failed with token=secret"}
	}
	return transport.ActionResponse{OK: true, Data: map[string]any{"id": id, "body_text": "safe explicit body"}}
}

func (batchBodyLeakyErrorTransport) DryRun(context.Context, transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{}
}

type paginatedSearchTransport struct{}

func (paginatedSearchTransport) Name() string {
	return "graph"
}

func (paginatedSearchTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (paginatedSearchTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{}
}

func (paginatedSearchTransport) Execute(context.Context, transport.ActionRequest) transport.ActionResponse {
	return transport.ActionResponse{
		OK: true,
		Data: map[string]any{
			"messages":  []any{map[string]any{"subject": "Planning"}},
			"returned":  1,
			"limit":     1,
			"truncated": true,
			"next_link": "https://graph.example.test/v1.0/me/messages?$skiptoken=next",
		},
	}
}

func (paginatedSearchTransport) DryRun(context.Context, transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{}
}

type paginatedSearchNextTransport struct {
	nextLinkUsed string
	queryUsed    string
	failNext     bool
}

func (client *paginatedSearchNextTransport) Name() string {
	return "graph"
}

func (client *paginatedSearchNextTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (client *paginatedSearchNextTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{}
}

func (client *paginatedSearchNextTransport) Execute(_ context.Context, request transport.ActionRequest) transport.ActionResponse {
	switch request.Name {
	case "mail.search":
		return transport.ActionResponse{OK: true, Data: map[string]any{
			"messages":  []any{map[string]any{"subject": "First"}},
			"returned":  1,
			"limit":     1,
			"truncated": true,
			"next_link": "https://graph.example.test/v1.0/me/messages?$skiptoken=next",
		}}
	case "mail.search_next":
		client.nextLinkUsed, _ = request.Payload["next_link"].(string)
		client.queryUsed, _ = request.Payload["query"].(string)
		if client.failNext {
			client.failNext = false
			return transport.ActionResponse{OK: false, Error: "temporary provider failure"}
		}
		return transport.ActionResponse{OK: true, Data: map[string]any{
			"messages":  []any{map[string]any{"subject": "Second"}},
			"returned":  1,
			"limit":     1,
			"truncated": false,
		}}
	default:
		return transport.ActionResponse{OK: false, Error: "unexpected action"}
	}
}

func (client *paginatedSearchNextTransport) DryRun(context.Context, transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{}
}

type blockingSearchNextTransport struct {
	firstSearchNextStarted  chan struct{}
	secondSearchNextStarted chan struct{}
	releaseSearchNext       chan struct{}
	releaseOnce             sync.Once
	mu                      sync.Mutex
	searchNextCalls         int
}

func newBlockingSearchNextTransport() *blockingSearchNextTransport {
	return &blockingSearchNextTransport{
		firstSearchNextStarted:  make(chan struct{}),
		secondSearchNextStarted: make(chan struct{}),
		releaseSearchNext:       make(chan struct{}),
	}
}

func (client *blockingSearchNextTransport) Name() string {
	return "graph"
}

func (client *blockingSearchNextTransport) Authenticate(context.Context, string) transport.AuthResult {
	return transport.AuthResult{OK: true}
}

func (client *blockingSearchNextTransport) Capabilities(context.Context) transport.CapabilitySet {
	return transport.CapabilitySet{}
}

func (client *blockingSearchNextTransport) Execute(_ context.Context, request transport.ActionRequest) transport.ActionResponse {
	switch request.Name {
	case "mail.search":
		return transport.ActionResponse{OK: true, Data: map[string]any{
			"messages":  []any{map[string]any{"subject": "First"}},
			"returned":  1,
			"limit":     1,
			"truncated": true,
			"next_link": "https://graph.example.test/v1.0/me/messages?$skiptoken=next",
		}}
	case "mail.search_next":
		callNumber := client.recordSearchNextCall()
		if callNumber == 1 {
			close(client.firstSearchNextStarted)
		}
		if callNumber == 2 {
			close(client.secondSearchNextStarted)
		}
		<-client.releaseSearchNext
		return transport.ActionResponse{OK: true, Data: map[string]any{
			"messages":  []any{map[string]any{"subject": "Second"}},
			"returned":  1,
			"limit":     1,
			"truncated": false,
		}}
	default:
		return transport.ActionResponse{OK: false, Error: "unexpected action"}
	}
}

func (client *blockingSearchNextTransport) DryRun(context.Context, transport.ActionRequest) transport.DryRunSummary {
	return transport.DryRunSummary{}
}

func (client *blockingSearchNextTransport) recordSearchNextCall() int {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.searchNextCalls++
	return client.searchNextCalls
}

func (client *blockingSearchNextTransport) searchNextCallCount() int {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.searchNextCalls
}

func (client *blockingSearchNextTransport) release() {
	client.releaseOnce.Do(func() {
		close(client.releaseSearchNext)
	})
}
