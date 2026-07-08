package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/dtonair/confluence-cli/config"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// resetFlags clears flag values and their `Changed` markers across the command
// tree. cobra reuses the rootCmd singleton between Execute() calls, so flag
// state otherwise leaks from one test into the next.
func resetFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
	for _, sub := range cmd.Commands() {
		resetFlags(sub)
	}
}

func jsonResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

// run executes the root command with args, capturing stdout. The stub transport
// (may be nil) handles HTTP and can record requests. Config is sandboxed to a
// temp CONFLUENCE_CONFIG so tests never read the developer's real file.
func run(t *testing.T, transport roundTripFunc, args ...string) (string, error) {
	t.Helper()

	t.Setenv("CONFLUENCE_CONFIG", t.TempDir()+"/confluence-cli.yaml")
	t.Setenv("CONFLUENCE_EMAIL", "dev@example.com")
	t.Setenv("CONFLUENCE_API_TOKEN", "token")
	t.Setenv("CONFLUENCE_SITE", "acme.atlassian.net")
	t.Setenv("CONFLUENCE_DEFAULT_SPACE", "")

	// Reset global flag state between runs.
	flagSpace, flagPretty = "", false
	resetFlags(rootCmd)
	testTransport = transport
	t.Cleanup(func() { testTransport = nil })

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs(args)
	err := rootCmd.Execute()

	w.Close()
	os.Stdout = origStdout
	out, _ := io.ReadAll(r)
	return string(out), err
}

func TestStatusJSON(t *testing.T) {
	out, err := run(t, nil, "status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("not json: %v\n%s", err, out)
	}
	if m["configured"] != true || m["site"] != "acme.atlassian.net" {
		t.Fatalf("unexpected status: %v", m)
	}
}

func TestStatusMissingSiteFails(t *testing.T) {
	t.Setenv("CONFLUENCE_CONFIG", t.TempDir()+"/missing.yaml")
	t.Setenv("CONFLUENCE_EMAIL", "dev@example.com")
	t.Setenv("CONFLUENCE_API_TOKEN", "token")
	t.Setenv("CONFLUENCE_SITE", "")
	t.Setenv("CONFLUENCE_DEFAULT_SPACE", "")

	flagSpace, flagPretty = "", false
	resetFlags(rootCmd)
	rootCmd.SetArgs([]string{"status"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected error when site missing")
	}
}

func TestPageListNoSpaceHitsPagesEndpoint(t *testing.T) {
	var gotURL string
	out, err := run(t, func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		return jsonResp(200, `{"results":[{"id":"1","title":"A","status":"current"},{"id":"2","title":"B","status":"current"}],"_links":{}}`), nil
	}, "page", "list", "--limit", "5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotURL, "/wiki/api/v2/pages") {
		t.Fatalf("unexpected path: %s", gotURL)
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(out), &arr); err != nil {
		t.Fatalf("not json array: %v\n%s", err, out)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(arr))
	}
}

func TestPageListWithSpaceResolvesKey(t *testing.T) {
	var urls []string
	_, err := run(t, func(r *http.Request) (*http.Response, error) {
		urls = append(urls, r.URL.String())
		if strings.Contains(r.URL.Path, "/spaces") && r.URL.Query().Get("keys") != "" {
			return jsonResp(200, `{"results":[{"id":"777","key":"ENG"}],"_links":{}}`), nil
		}
		return jsonResp(200, `{"results":[{"id":"1","title":"A","status":"current","spaceId":"777"}],"_links":{}}`), nil
	}, "page", "list", "--space", "ENG")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(urls) != 2 {
		t.Fatalf("expected key lookup + list (2 calls), got %d: %v", len(urls), urls)
	}
	if !strings.Contains(urls[0], "/spaces?keys=ENG") {
		t.Fatalf("first call not a key lookup: %s", urls[0])
	}
	if !strings.Contains(urls[1], "/spaces/777/pages") {
		t.Fatalf("second call not scoped to resolved space id: %s", urls[1])
	}
}

func TestPageGetBodyFormat(t *testing.T) {
	var gotURL string
	out, err := run(t, func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		return jsonResp(200, `{"id":"55","title":"Runbook","status":"current","version":{"number":3}}`), nil
	}, "page", "get", "55", "--body-format", "atlas_doc_format", "--pretty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotURL, "/pages/55") || !strings.Contains(gotURL, "body-format=atlas_doc_format") {
		t.Fatalf("unexpected url: %s", gotURL)
	}
	if strings.TrimSpace(out) != "55 Runbook [current] v3" {
		t.Fatalf("unexpected pretty output: %q", out)
	}
}

func TestPageGetInvalidID(t *testing.T) {
	_, err := run(t, nil, "page", "get", "abc")
	if err == nil {
		t.Fatal("expected error for non-numeric id")
	}
}

func TestPageGetRejectsBadBodyFormat(t *testing.T) {
	called := false
	_, err := run(t, func(r *http.Request) (*http.Response, error) {
		called = true
		return jsonResp(200, `{}`), nil
	}, "page", "get", "55", "--body-format", "markdown")
	if err == nil {
		t.Fatal("expected error for invalid body-format")
	}
	if called {
		t.Fatal("should not call API for invalid body-format")
	}
}

func TestSpaceGetByKeyResolvesThenFetches(t *testing.T) {
	var urls []string
	out, err := run(t, func(r *http.Request) (*http.Response, error) {
		urls = append(urls, r.URL.String())
		if r.URL.Query().Get("keys") != "" {
			return jsonResp(200, `{"results":[{"id":"777","key":"ENG","name":"Engineering","type":"global"}],"_links":{}}`), nil
		}
		return jsonResp(200, `{"id":"777","key":"ENG","name":"Engineering","type":"global"}`), nil
	}, "space", "get", "ENG", "--pretty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(urls) != 2 || !strings.Contains(urls[1], "/spaces/777") {
		t.Fatalf("expected resolve then /spaces/777, got %v", urls)
	}
	if strings.TrimSpace(out) != "ENG — Engineering [global]" {
		t.Fatalf("unexpected pretty output: %q", out)
	}
}

func TestPageCreatePostsBody(t *testing.T) {
	var gotURL, gotMethod string
	var gotBody []byte
	out, err := run(t, func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		gotMethod = r.Method
		gotBody, _ = io.ReadAll(r.Body)
		return jsonResp(200, `{"id":"900","title":"New","status":"current","spaceId":"777"}`), nil
	}, "page", "create", "--space", "777", "--title", "New", "--body", "<p>hi</p>", "--pretty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost || !strings.HasSuffix(gotURL, "/wiki/api/v2/pages") {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotURL)
	}
	var sent map[string]any
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatalf("body not json: %v", err)
	}
	if sent["spaceId"] != "777" || sent["title"] != "New" || sent["status"] != "current" {
		t.Fatalf("unexpected body: %s", gotBody)
	}
	body := sent["body"].(map[string]any)
	if body["representation"] != "storage" || body["value"] != "<p>hi</p>" {
		t.Fatalf("unexpected body payload: %s", gotBody)
	}
	if strings.TrimSpace(out) != "900 New [current] (space 777)" {
		t.Fatalf("unexpected pretty output: %q", out)
	}
}

func TestPageUpdateBumpsVersionAndCarriesFields(t *testing.T) {
	var methods []string
	var putBody []byte
	_, err := run(t, func(r *http.Request) (*http.Response, error) {
		methods = append(methods, r.Method)
		if r.Method == http.MethodGet {
			return jsonResp(200, `{"id":"55","title":"Old Title","status":"current","version":{"number":3},"body":{"storage":{"value":"<p>old</p>","representation":"storage"}}}`), nil
		}
		putBody, _ = io.ReadAll(r.Body)
		return jsonResp(200, `{"id":"55","title":"Old Title","status":"current","version":{"number":4}}`), nil
	}, "page", "update", "55", "--body", "<p>new</p>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(methods) != 2 || methods[0] != http.MethodGet || methods[1] != http.MethodPut {
		t.Fatalf("expected GET then PUT, got %v", methods)
	}
	var sent map[string]any
	if err := json.Unmarshal(putBody, &sent); err != nil {
		t.Fatalf("put body not json: %v", err)
	}
	version := sent["version"].(map[string]any)
	if version["number"].(float64) != 4 {
		t.Fatalf("expected version bumped to 4, got %v", version["number"])
	}
	// Title not changed → carried over from the GET.
	if sent["title"] != "Old Title" {
		t.Fatalf("title not carried over: %s", putBody)
	}
	// Body changed → new value sent.
	body := sent["body"].(map[string]any)
	if body["value"] != "<p>new</p>" {
		t.Fatalf("body not updated: %s", putBody)
	}
}

func TestPageUpdateNoFieldsFails(t *testing.T) {
	called := false
	_, err := run(t, func(r *http.Request) (*http.Response, error) {
		called = true
		return jsonResp(200, `{}`), nil
	}, "page", "update", "55")
	if err == nil {
		t.Fatal("expected error when no update fields given")
	}
	if called {
		t.Fatal("should not call API when nothing to update")
	}
}

func TestCommentCreatePostsBody(t *testing.T) {
	var gotURL, gotMethod string
	var gotBody []byte
	out, err := run(t, func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		gotMethod = r.Method
		gotBody, _ = io.ReadAll(r.Body)
		return jsonResp(200, `{"id":"321","body":{"storage":{"value":"Looks good"}}}`), nil
	}, "comment", "create", "--page", "55", "--body", "Looks good", "--pretty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost || !strings.HasSuffix(gotURL, "/wiki/api/v2/footer-comments") {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotURL)
	}
	var sent map[string]any
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatalf("body not json: %v", err)
	}
	if sent["pageId"] != "55" {
		t.Fatalf("unexpected pageId: %s", gotBody)
	}
	if _, has := sent["parentCommentId"]; has {
		t.Fatalf("top-level comment must not send parentCommentId: %s", gotBody)
	}
	if strings.TrimSpace(out) != "#321: Looks good" {
		t.Fatalf("unexpected pretty output: %q", out)
	}
}

func TestCommentReplyPostsParent(t *testing.T) {
	var gotBody []byte
	_, err := run(t, func(r *http.Request) (*http.Response, error) {
		gotBody, _ = io.ReadAll(r.Body)
		return jsonResp(200, `{"id":"322","body":{"storage":{"value":"Fixed"}}}`), nil
	}, "comment", "create", "--page", "55", "--body", "Fixed", "--reply-to", "321")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var sent map[string]any
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatalf("body not json: %v", err)
	}
	if sent["parentCommentId"] != "321" {
		t.Fatalf("expected parentCommentId 321: %s", gotBody)
	}
}

func TestCommentEmptyBodyFails(t *testing.T) {
	called := false
	_, err := run(t, func(r *http.Request) (*http.Response, error) {
		called = true
		return jsonResp(200, `{}`), nil
	}, "comment", "create", "--page", "55", "--body", "   ")
	if err == nil {
		t.Fatal("expected error for empty body")
	}
	if called {
		t.Fatal("should not call API for empty body")
	}
}

func TestConfigListRedactsToken(t *testing.T) {
	cfgPath := t.TempDir() + "/confluence-cli.yaml"
	if err := config.SetFileValue(cfgPath, "api_token", "super-secret"); err != nil {
		t.Fatalf("seed token: %v", err)
	}
	if err := config.SetFileValue(cfgPath, "email", "u@example.com"); err != nil {
		t.Fatalf("seed email: %v", err)
	}

	t.Setenv("CONFLUENCE_CONFIG", cfgPath)
	flagSpace, flagPretty = "", false
	resetFlags(rootCmd)

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	rootCmd.SetArgs([]string{"config", "list"})
	err := rootCmd.Execute()
	w.Close()
	os.Stdout = origStdout
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatalf("config list failed: %v", err)
	}
	if strings.Contains(string(out), "super-secret") {
		t.Fatalf("token leaked in config list output: %s", out)
	}
	if !strings.Contains(string(out), "***redacted***") {
		t.Fatalf("token not redacted: %s", out)
	}
}
