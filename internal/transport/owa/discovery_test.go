package owa_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/johnkil/outlook-agent/internal/policy"
	"github.com/johnkil/outlook-agent/internal/secret"
	"github.com/johnkil/outlook-agent/internal/transport/owa"
)

func TestDiscoverServiceActionsFromTextExtractsOWAPatterns(t *testing.T) {
	text := `
		fetch("/owa/service.svc?action=FindItem");
		const url = "/owa/service.svc?action=GetCalendarView&foo=bar";
		const requestType = "GetUserAvailabilityInternalJsonRequest:#Exchange";
		const headers = {"Action": "FindPeople"};
		const ignoredLower = {"action": "canary"};
		const ignoredQueryLower = "/owa/service.svc?action=canary";
		const ignoredVariable = "/owa/service.svc?action=${action}";
		fetch("/owa/service.svc?action=FindItem");
	`

	actions := owa.DiscoverServiceActions(text)

	expected := []string{
		"FindItem",
		"FindPeople",
		"GetCalendarView",
		"GetUserAvailabilityInternal",
	}
	if !slices.Equal(actions, expected) {
		t.Fatalf("expected discovered actions %#v, got %#v", expected, actions)
	}
}

func TestDiscoverLinkedScriptSourcesExtractsUniqueScriptSrcs(t *testing.T) {
	html := `
		<script src="/owa/scripts/boot.js"></script>
		<script defer src='/owa/scripts/app.js?v=1'></script>
		<script>console.log("inline")</script>
		<script src="/owa/scripts/boot.js"></script>
	`

	sources := owa.DiscoverLinkedScriptSources(html)

	expected := []string{"/owa/scripts/app.js?v=1", "/owa/scripts/boot.js"}
	if !slices.Equal(sources, expected) {
		t.Fatalf("expected script sources %#v, got %#v", expected, sources)
	}
}

func TestDiscoverLinkedScriptSourcesExtractsQuotedJavaScriptReferences(t *testing.T) {
	html := `
		<script>boot("/owa/prem/boot.js?ver=1")</script>
		<script>var next = 'scripts/app.js';</script>
		<script>var ignored = "/owa/prem/theme.css";</script>
		<script>var duplicate = "/owa/prem/boot.js?ver=1";</script>
	`

	sources := owa.DiscoverLinkedScriptSources(html)

	expected := []string{"/owa/prem/boot.js?ver=1", "scripts/app.js"}
	if !slices.Equal(sources, expected) {
		t.Fatalf("expected quoted script sources %#v, got %#v", expected, sources)
	}
}

func TestDiscoverNavigationHintSourcesExtractsMetaRefreshAndLocationTargets(t *testing.T) {
	html := `
		<meta http-equiv="refresh" content="0; URL=/owa/bootstrap.aspx?layout=1">
		<script>window.location = '/owa/deeplink.aspx?mode=full';</script>
		<script>location.replace("shell/start.aspx");</script>
		<script>var ignored = "/owa/scripts/app.js";</script>
	`

	sources := owa.DiscoverNavigationHintSources(html)

	expected := []string{"/owa/bootstrap.aspx?layout=1", "/owa/deeplink.aspx?mode=full", "shell/start.aspx"}
	if !slices.Equal(sources, expected) {
		t.Fatalf("expected navigation hints %#v, got %#v", expected, sources)
	}
}

func TestCompareDiscoveredServiceActionsReportsUnknownAndMissing(t *testing.T) {
	discovered := []string{
		"FindItem",
		"GetAttachment",
		"SendItem",
		"TotallyNewAction",
	}

	report := owa.CompareDiscoveredServiceActions(discovered)

	for _, expected := range []string{"FindItem", "GetAttachment", "SendItem"} {
		if !slices.Contains(report.Classified, expected) {
			t.Fatalf("expected %s in classified report %#v", expected, report)
		}
	}
	if !slices.Equal(report.Unknown, []string{"TotallyNewAction"}) {
		t.Fatalf("expected unknown action, got %#v", report.Unknown)
	}
	if !slices.Contains(report.MissingClassified, "ArchiveItem") {
		t.Fatalf("expected missing classified registry action in report %#v", report)
	}
	if report.Classes["SendItem"] != policy.SendLike {
		t.Fatalf("expected SendItem class send_like, got %#v", report.Classes)
	}
	if report.Classes["TotallyNewAction"] != policy.Unknown {
		t.Fatalf("expected unknown class for new action, got %#v", report.Classes)
	}
}

func TestCompareDiscoveredServiceActionsReturnsEmptySlicesForNoFindings(t *testing.T) {
	report := owa.CompareDiscoveredServiceActions(nil)

	if report.Discovered == nil || report.Classified == nil || report.Unknown == nil {
		t.Fatalf("expected empty slices instead of nil slices: %#v", report)
	}
	if len(report.Discovered) != 0 || len(report.Classified) != 0 || len(report.Unknown) != 0 {
		t.Fatalf("expected no findings, got %#v", report)
	}
}

func TestTransportDiscoversActionsFromLinkedScripts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/":
			response.Header().Set("Content-Type", "text/html")
			_, _ = response.Write([]byte(`
				<html>
					<script src="/owa/scripts/boot.js"></script>
					<script src="scripts/app.js?v=1"></script>
				</html>
			`))
		case "/owa/scripts/boot.js":
			response.Header().Set("Content-Type", "application/javascript")
			_, _ = response.Write([]byte(`fetch("/owa/service.svc?action=FindItem");`))
		case "/owa/scripts/app.js":
			if request.URL.Query().Get("v") != "1" {
				t.Fatalf("expected script query to be preserved, got %s", request.URL.RawQuery)
			}
			response.Header().Set("Content-Type", "application/javascript")
			_, _ = response.Write([]byte(`const requestType = "GetAttachmentJsonRequest:#Exchange";`))
		default:
			t.Fatalf("unexpected path: %s", request.URL.String())
		}
	}))
	defer server.Close()
	client := owa.NewTransport(owa.Config{
		BaseURL:   server.URL,
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())

	actions, err := client.DiscoverServiceActionsFromURLWithOptions(context.Background(), "/owa/", owa.DiscoveryOptions{IncludeLinkedScripts: true})

	if err != nil {
		t.Fatalf("discover linked script actions: %v", err)
	}
	expected := []string{"FindItem", "GetAttachment"}
	if !slices.Equal(actions, expected) {
		t.Fatalf("expected linked script actions %#v, got %#v", expected, actions)
	}
}

func TestTransportReportsDiscoveryDiagnostics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/":
			response.Header().Set("Content-Type", "text/html")
			_, _ = response.Write([]byte(`<script src="/owa/scripts/app.js"></script>`))
		case "/owa/scripts/app.js":
			response.Header().Set("Content-Type", "application/javascript")
			_, _ = response.Write([]byte(`fetch("/owa/service.svc?action=FindItem");`))
		default:
			t.Fatalf("unexpected path: %s", request.URL.String())
		}
	}))
	defer server.Close()
	client := owa.NewTransport(owa.Config{
		BaseURL:   server.URL,
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())

	diagnostics, err := client.DiscoverServiceActionsFromURLDiagnostics(context.Background(), "/owa/", owa.DiscoveryOptions{IncludeLinkedScripts: true})

	if err != nil {
		t.Fatalf("discover diagnostics: %v", err)
	}
	expectedActions := []string{"FindItem"}
	if !slices.Equal(diagnostics.Actions, expectedActions) {
		t.Fatalf("expected actions %#v, got %#v", expectedActions, diagnostics.Actions)
	}
	if len(diagnostics.Sources) != 2 {
		t.Fatalf("expected page and script source diagnostics, got %#v", diagnostics.Sources)
	}
	if diagnostics.Sources[0].Source != "/owa/" {
		t.Fatalf("expected root source first, got %#v", diagnostics.Sources)
	}
	if diagnostics.Sources[0].Status != 200 || diagnostics.Sources[0].ContentType != "text/html" {
		t.Fatalf("expected root status/content type diagnostics, got %#v", diagnostics.Sources[0])
	}
	if diagnostics.Sources[0].LinkedScripts != 1 || diagnostics.Sources[0].Actions != 0 {
		t.Fatalf("unexpected root diagnostics: %#v", diagnostics.Sources[0])
	}
	if diagnostics.Sources[1].Source != "/owa/scripts/app.js" {
		t.Fatalf("expected linked script source second, got %#v", diagnostics.Sources)
	}
	if diagnostics.Sources[1].Actions != 1 {
		t.Fatalf("unexpected linked script diagnostics: %#v", diagnostics.Sources[1])
	}
}

func TestTransportDiscoveryDiagnosticsReportsFinalPathTitleMarkerAndScriptBlocks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/start":
			http.Redirect(response, request, "/owa/final?layout=1", http.StatusFound)
		case "/owa/final":
			response.Header().Set("Content-Type", "text/html")
			_, _ = response.Write([]byte(`<html><head><title>Outlook</title></head><body><script>boot()</script></body></html>`))
		default:
			t.Fatalf("unexpected path: %s", request.URL.String())
		}
	}))
	defer server.Close()
	client := owa.NewTransport(owa.Config{
		BaseURL:   server.URL,
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())

	diagnostics, err := client.DiscoverServiceActionsFromURLDiagnostics(context.Background(), "/owa/start", owa.DiscoveryOptions{})

	if err != nil {
		t.Fatalf("discover diagnostics: %v", err)
	}
	if len(diagnostics.Sources) != 1 {
		t.Fatalf("expected one source diagnostic, got %#v", diagnostics.Sources)
	}
	source := diagnostics.Sources[0]
	if source.FinalPath != "/owa/final?layout=1" {
		t.Fatalf("expected sanitized final path, got %#v", source)
	}
	if !source.FinalPathChanged {
		t.Fatalf("expected final path changed marker, got %#v", source)
	}
	if !source.TitlePresent || source.TitleKind != "outlook" {
		t.Fatalf("expected sanitized Outlook title marker, got %#v", source)
	}
	if source.ScriptBlocks != 1 {
		t.Fatalf("expected one inline script block, got %#v", source)
	}
}

func TestTransportDiscoveryDiagnosticsReportsSanitizedTargetPreviews(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/start/page.aspx":
			response.Header().Set("Content-Type", "text/html")
			_, _ = response.Write([]byte(`
				<html>
					<script src="../scripts/app.js?v=1"></script>
					<script>
						const boot = "/owa/prem/15.2.1748/scripts/boot.js";
						const ignored = "https://other.example.test/owa/scripts/evil.js";
						window.location = "shell/start.aspx";
					</script>
					<meta http-equiv="refresh" content="0; URL=/owa/bootstrap.aspx?layout=1">
				</html>
			`))
		default:
			t.Fatalf("unexpected path: %s", request.URL.String())
		}
	}))
	defer server.Close()
	client := owa.NewTransport(owa.Config{
		BaseURL:   server.URL,
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())

	diagnostics, err := client.DiscoverServiceActionsFromURLDiagnostics(context.Background(), "/owa/start/page.aspx", owa.DiscoveryOptions{})

	if err != nil {
		t.Fatalf("discover diagnostics: %v", err)
	}
	if len(diagnostics.Sources) != 1 {
		t.Fatalf("expected one source diagnostic, got %#v", diagnostics.Sources)
	}
	expectedScripts := []string{"/owa/prem/15.2.1748/scripts/boot.js", "/owa/scripts/app.js?v=1"}
	if !slices.Equal(diagnostics.Sources[0].LinkedScriptPaths, expectedScripts) {
		t.Fatalf("expected sanitized linked script paths %#v, got %#v", expectedScripts, diagnostics.Sources[0])
	}
	expectedNavigation := []string{"/owa/bootstrap.aspx?layout=1", "/owa/start/shell/start.aspx"}
	if !slices.Equal(diagnostics.Sources[0].NavigationHintPaths, expectedNavigation) {
		t.Fatalf("expected sanitized navigation hint paths %#v, got %#v", expectedNavigation, diagnostics.Sources[0])
	}
}

func TestTransportFollowsNavigationHints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/":
			response.Header().Set("Content-Type", "text/html")
			_, _ = response.Write([]byte(`<meta http-equiv="refresh" content="0; URL=/owa/bootstrap.aspx">`))
		case "/owa/bootstrap.aspx":
			response.Header().Set("Content-Type", "text/html")
			_, _ = response.Write([]byte(`<script src="scripts/app.js?v=1"></script>`))
		case "/owa/scripts/app.js":
			if request.URL.Query().Get("v") != "1" {
				t.Fatalf("expected script query to be preserved, got %s", request.URL.RawQuery)
			}
			response.Header().Set("Content-Type", "application/javascript")
			_, _ = response.Write([]byte(`fetch("/owa/service.svc?action=FindItem");`))
		default:
			t.Fatalf("unexpected path: %s", request.URL.String())
		}
	}))
	defer server.Close()
	client := owa.NewTransport(owa.Config{
		BaseURL:   server.URL,
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())

	diagnostics, err := client.DiscoverServiceActionsFromURLDiagnostics(context.Background(), "/owa/", owa.DiscoveryOptions{
		FollowNavigationHints: true,
		IncludeLinkedScripts:  true,
	})

	if err != nil {
		t.Fatalf("discover diagnostics with navigation hints: %v", err)
	}
	expectedActions := []string{"FindItem"}
	if !slices.Equal(diagnostics.Actions, expectedActions) {
		t.Fatalf("expected actions %#v, got %#v", expectedActions, diagnostics.Actions)
	}
	if len(diagnostics.Sources) != 3 {
		t.Fatalf("expected root, navigation target, and script diagnostics, got %#v", diagnostics.Sources)
	}
	if diagnostics.Sources[0].NavigationHints != 1 {
		t.Fatalf("expected root navigation hint count, got %#v", diagnostics.Sources[0])
	}
	if diagnostics.Sources[1].Source != "/owa/bootstrap.aspx" || diagnostics.Sources[1].LinkedScripts != 1 {
		t.Fatalf("unexpected navigation target diagnostics: %#v", diagnostics.Sources[1])
	}
	if diagnostics.Sources[2].Source != "scripts/app.js?v=1" || diagnostics.Sources[2].Actions != 1 {
		t.Fatalf("unexpected linked script diagnostics: %#v", diagnostics.Sources[2])
	}
}

func TestTransportDiscoveryDiagnosticsDetectsLogonPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/":
			response.Header().Set("Content-Type", "text/html")
			_, _ = response.Write([]byte(`<form action="/owa/auth.owa"><input name="username"><input name="password"></form>`))
		default:
			t.Fatalf("unexpected path: %s", request.URL.String())
		}
	}))
	defer server.Close()
	client := owa.NewTransport(owa.Config{
		BaseURL:   server.URL,
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())

	diagnostics, err := client.DiscoverServiceActionsFromURLDiagnostics(context.Background(), "/owa/", owa.DiscoveryOptions{})

	if err != nil {
		t.Fatalf("discover diagnostics: %v", err)
	}
	if len(diagnostics.Sources) != 1 {
		t.Fatalf("expected one source diagnostic, got %#v", diagnostics.Sources)
	}
	if !diagnostics.Sources[0].LooksLikeLogonPage {
		t.Fatalf("expected logon page marker in diagnostics: %#v", diagnostics.Sources[0])
	}
}

func TestTransportDiscoveryDiagnosticsDetectsOWAErrorPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/":
			response.Header().Set("Content-Type", "text/html")
			_, _ = response.Write([]byte(`
				<html>
					<link rel="stylesheet" href="/owa/15.2.1748.10/themes/resources/error2.css">
					<img src="/owa/15.2.1748.10/themes/base/errorBG.gif">
				</html>
			`))
		default:
			t.Fatalf("unexpected path: %s", request.URL.String())
		}
	}))
	defer server.Close()
	client := owa.NewTransport(owa.Config{
		BaseURL:   server.URL,
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())

	diagnostics, err := client.DiscoverServiceActionsFromURLDiagnostics(context.Background(), "/owa/", owa.DiscoveryOptions{})

	if err != nil {
		t.Fatalf("discover diagnostics: %v", err)
	}
	if len(diagnostics.Sources) != 1 {
		t.Fatalf("expected one source diagnostic, got %#v", diagnostics.Sources)
	}
	if !diagnostics.Sources[0].LooksLikeOWAErrorPage {
		t.Fatalf("expected OWA error-page marker in diagnostics: %#v", diagnostics.Sources[0])
	}
}

func TestTransportDiscoveryDiagnosticsCanReportHTTPStatusErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/missing.js":
			response.Header().Set("Content-Type", "text/html")
			response.WriteHeader(http.StatusNotFound)
		default:
			t.Fatalf("unexpected path: %s", request.URL.String())
		}
	}))
	defer server.Close()
	client := owa.NewTransport(owa.Config{
		BaseURL:   server.URL,
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())

	diagnostics, err := client.DiscoverServiceActionsFromURLDiagnostics(context.Background(), "/owa/missing.js", owa.DiscoveryOptions{
		ContinueOnHTTPError: true,
	})

	if err != nil {
		t.Fatalf("discover diagnostics should continue after HTTP status error: %v", err)
	}
	if len(diagnostics.Actions) != 0 {
		t.Fatalf("expected no discovered actions, got %#v", diagnostics.Actions)
	}
	if len(diagnostics.Sources) != 1 {
		t.Fatalf("expected one source diagnostic, got %#v", diagnostics.Sources)
	}
	source := diagnostics.Sources[0]
	if source.Status != http.StatusNotFound || source.FinalPath != "/owa/missing.js" || source.FetchError != "http_status" {
		t.Fatalf("expected sanitized HTTP status diagnostic, got %#v", source)
	}
}

func TestTransportDiscoversActionsFromAuthenticatedURL(t *testing.T) {
	var sawSessionCookie bool
	var sawCanaryHeader bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/owa/auth.owa":
			http.SetCookie(response, &http.Cookie{Name: "X-OWA-CANARY", Value: "canary-secret"})
			response.WriteHeader(http.StatusOK)
		case "/owa/scripts/app.js":
			if cookie, err := request.Cookie("X-OWA-CANARY"); err == nil && cookie.Value == "canary-secret" {
				sawSessionCookie = true
			}
			sawCanaryHeader = request.Header.Get("X-OWA-CANARY") == "canary-secret"
			response.Header().Set("Content-Type", "application/javascript")
			_, _ = response.Write([]byte(`
				fetch("/owa/service.svc?action=FindItem");
				const requestType = "GetAttachmentJsonRequest:#Exchange";
				const payload = {"Action":"TotallyNewAction"};
			`))
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()
	client := owa.NewTransport(owa.Config{
		BaseURL:   server.URL,
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), server.Client())

	actions, err := client.DiscoverServiceActionsFromURL(context.Background(), "/owa/scripts/app.js")

	if err != nil {
		t.Fatalf("discover authenticated actions: %v", err)
	}
	expected := []string{"FindItem", "GetAttachment", "TotallyNewAction"}
	if !slices.Equal(actions, expected) {
		t.Fatalf("expected discovered actions %#v, got %#v", expected, actions)
	}
	if !sawSessionCookie {
		t.Fatal("expected authenticated discovery request to include session cookie")
	}
	if !sawCanaryHeader {
		t.Fatal("expected authenticated discovery request to include canary header")
	}
}

func TestTransportDiscoveryRejectsCrossOriginURL(t *testing.T) {
	client := owa.NewTransport(owa.Config{
		BaseURL:   "https://example.test",
		Username:  "DOMAIN\\user",
		SecretRef: secret.Ref("memory:owa"),
	}, secret.NewMemoryStore(map[string]string{"memory:owa": "password"}), nil)

	_, err := client.DiscoverServiceActionsFromURL(context.Background(), "https://other.example.test/owa/app.js")

	if err == nil {
		t.Fatal("expected cross-origin discovery URL to be rejected")
	}
}
