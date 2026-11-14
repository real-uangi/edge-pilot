package infra

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/real-uangi/edge-pilot/internal/shared/dto"
)

type registryRoundTripFunc func(*http.Request) (*http.Response, error)

func (f registryRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func registryResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestRegistryTagsEndpoint(t *testing.T) {
	for _, tc := range []struct {
		repo string
		want string
	}{
		{"nginx", "https://registry-1.docker.io/v2/library/nginx/tags/list"},
		{"my.image", "https://registry-1.docker.io/v2/library/my.image/tags/list"},
		{"docker.io/nginx", "https://registry-1.docker.io/v2/library/nginx/tags/list"},
		{"index.docker.io/team/app", "https://registry-1.docker.io/v2/team/app/tags/list"},
		{"team/app", "https://registry-1.docker.io/v2/team/app/tags/list"},
		{"ghcr.io/team/app", "https://ghcr.io/v2/team/app/tags/list"},
		{"harbor.example.com:5443/team/app", "https://harbor.example.com:5443/v2/team/app/tags/list"},
	} {
		t.Run(tc.repo, func(t *testing.T) {
			endpoint, _, err := registryTagsEndpoint(tc.repo)
			if err != nil || endpoint.String() != tc.want {
				t.Fatalf("registryTagsEndpoint() = %v, %v; want %s", endpoint, err, tc.want)
			}
		})
	}
	for _, repo := range []string{"", "http://registry.example.com/team/app", "ghcr.io/team/app:latest", "ghcr.io/team/app@sha256:abc", "ghcr.io/../app", "ghcr.io/team/app?x=y", "user@ghcr.io/team/app", "ghcr.io/Team/app"} {
		if _, _, err := registryTagsEndpoint(repo); err == nil {
			t.Errorf("registryTagsEndpoint(%q) should fail", repo)
		}
	}
}

func TestImageTagListerAnonymousPagination(t *testing.T) {
	requests := 0
	lister := &imageTagLister{client: &http.Client{Transport: registryRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.URL.String() != "https://registry-1.docker.io/v2/library/nginx/tags/list?last=v1&n=100" {
			t.Fatalf("unexpected URL %s", req.URL)
		}
		if req.Header.Get("Authorization") != "" {
			t.Fatal("anonymous request sent credentials")
		}
		deadline, ok := req.Context().Deadline()
		if !ok || time.Until(deadline) > 15*time.Second {
			t.Fatal("registry request is missing its 15-second deadline")
		}
		resp := registryResponse(http.StatusOK, `{"name":"library/nginx","tags":["v2","v3"]}`)
		resp.Header.Set("Link", `</v2/library/nginx/tags/list?n=100&last=v3>; rel="next"`)
		return resp, nil
	})}}
	output, err := lister.ListTags(context.Background(), "nginx", "v1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 1 || output.ImageRepo != "nginx" || len(output.Tags) != 2 || output.NextCursor != "v3" {
		t.Fatalf("unexpected output %#v; requests=%d", output, requests)
	}
}

func TestImageTagListerBasicAuthentication(t *testing.T) {
	requests := 0
	credentials := &dto.ResolvedRegistryCredential{Host: "harbor.example.com", Username: "robot", Secret: "private-secret"}
	lister := &imageTagLister{client: &http.Client{Transport: registryRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			resp := registryResponse(http.StatusUnauthorized, "")
			resp.Header.Set("WWW-Authenticate", `Basic realm="registry"`)
			return resp, nil
		}
		username, secret, ok := req.BasicAuth()
		if !ok || username != credentials.Username || secret != credentials.Secret {
			t.Fatal("request did not use resolved Basic credentials")
		}
		return registryResponse(http.StatusOK, `{"name":"team/app","tags":["latest"]}`), nil
	})}}
	output, err := lister.ListTags(context.Background(), "harbor.example.com/team/app", "", credentials)
	if err != nil || requests != 2 || len(output.Tags) != 1 {
		t.Fatalf("ListTags() = %#v, %v; requests=%d", output, err, requests)
	}
}

func TestImageTagListerBearerAuthentication(t *testing.T) {
	for _, authenticated := range []bool{false, true} {
		t.Run(map[bool]string{false: "anonymous", true: "private"}[authenticated], func(t *testing.T) {
			requests := 0
			var credential *dto.ResolvedRegistryCredential
			if authenticated {
				credential = &dto.ResolvedRegistryCredential{Host: "ghcr.io", Username: "robot", Secret: "private-secret"}
			}
			lister := &imageTagLister{client: &http.Client{Transport: registryRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				switch requests {
				case 1:
					resp := registryResponse(http.StatusUnauthorized, "")
					resp.Header.Set("WWW-Authenticate", `Bearer realm="https://auth.example.com/token",service="ghcr.io",scope="repository:team/app:pull,push"`)
					return resp, nil
				case 2:
					if req.URL.Host != "auth.example.com" || req.URL.Query().Get("scope") != "repository:team/app:pull" || req.URL.Query().Get("service") != "ghcr.io" {
						t.Fatalf("unexpected token URL %s", req.URL)
					}
					username, secret, ok := req.BasicAuth()
					if authenticated && (!ok || username != "robot" || secret != "private-secret") {
						t.Fatal("token request did not use credentials")
					}
					if !authenticated && ok {
						t.Fatal("anonymous token request sent credentials")
					}
					return registryResponse(http.StatusOK, `{"access_token":"repository-token"}`), nil
				case 3:
					if req.URL.Host != "ghcr.io" || req.Header.Get("Authorization") != "Bearer repository-token" {
						t.Fatal("registry request did not use bearer token")
					}
					return registryResponse(http.StatusOK, `{"name":"team/app","tags":["latest","v1"]}`), nil
				default:
					t.Fatalf("unexpected request %d", requests)
					return nil, nil
				}
			})}}
			output, err := lister.ListTags(context.Background(), "ghcr.io/team/app", "", credential)
			if err != nil || requests != 3 || len(output.Tags) != 2 {
				t.Fatalf("ListTags() = %#v, %v; requests=%d", output, err, requests)
			}
		})
	}
}

func TestRegistryNextCursorRejectsUntrustedLinks(t *testing.T) {
	endpoint, _ := url.Parse("https://ghcr.io/v2/team/app/tags/list?n=100")
	for _, link := range []string{
		`<https://evil.example.com/v2/team/app/tags/list?last=v2>; rel="next"`,
		`<http://ghcr.io/v2/team/app/tags/list?last=v2>; rel="next"`,
		`</v2/other/app/tags/list?last=v2>; rel="next"`,
		`<https://user:secret@ghcr.io/v2/team/app/tags/list?last=v2>; rel="next"`,
		`</v2/team/app/tags/list?last=../v2>; rel="next"`,
	} {
		if _, err := registryNextCursor(endpoint, []string{link}); err == nil {
			t.Errorf("registryNextCursor(%q) should fail", link)
		}
	}
	cursor, err := registryNextCursor(endpoint, []string{`</v2/team/app/tags/list?last=v1>; rel="prev", <?last=v3&n=10>; rel="next"`})
	if err != nil || cursor != "v3" {
		t.Fatalf("registryNextCursor() = %q, %v", cursor, err)
	}
}

func TestImageTagListerRejectsHTTPRealmAndDoesNotFollowRedirects(t *testing.T) {
	for _, challenge := range []string{`Bearer realm="http://auth.example.com/token"`, `Bearer realm="https://user:secret@auth.example.com/token"`} {
		requests := 0
		lister := NewImageTagLister().(*imageTagLister)
		lister.client.Transport = registryRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests++
			resp := registryResponse(http.StatusUnauthorized, "")
			resp.Header.Set("WWW-Authenticate", challenge)
			return resp, nil
		})
		if _, err := lister.ListTags(context.Background(), "ghcr.io/team/app", "", &dto.ResolvedRegistryCredential{Secret: "secret"}); err == nil || requests != 1 {
			t.Fatalf("invalid realm made %d requests or returned no error: %v", requests, err)
		}
	}
	requests := 0
	lister := NewImageTagLister().(*imageTagLister)
	lister.client.Transport = registryRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		resp := registryResponse(http.StatusTemporaryRedirect, "")
		resp.Header.Set("Location", "https://evil.example.com/collect")
		return resp, nil
	})
	if _, err := lister.ListTags(context.Background(), "ghcr.io/team/app", "", nil); err == nil || requests != 1 {
		t.Fatalf("redirect made %d requests or returned no error: %v", requests, err)
	}
}

func TestImageTagListerErrorsDoNotExposeUpstreamSecrets(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"forbidden", http.StatusForbidden, "private-secret"},
		{"missing", http.StatusNotFound, "private-secret"},
		{"limited", http.StatusTooManyRequests, "private-secret"},
		{"server", http.StatusInternalServerError, "private-secret"},
		{"malformed", http.StatusOK, "private-secret"},
		{"invalid tag", http.StatusOK, `{"name":"team/app","tags":["private-secret/invalid"]}`},
		{"missing name", http.StatusOK, `{}`},
		{"wrong repository", http.StatusOK, `{"name":"other/app","tags":["latest"]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lister := &imageTagLister{client: &http.Client{Transport: registryRoundTripFunc(func(*http.Request) (*http.Response, error) {
				return registryResponse(tc.status, tc.body), nil
			})}}
			_, err := lister.ListTags(context.Background(), "ghcr.io/team/app", "", nil)
			if err == nil || strings.Contains(err.Error(), "private-secret") {
				t.Fatalf("expected sanitized error, got %v", err)
			}
		})
	}
	lister := &imageTagLister{client: &http.Client{Transport: registryRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("private-secret")
	})}}
	if _, err := lister.ListTags(context.Background(), "ghcr.io/team/app", "", nil); err == nil || strings.Contains(err.Error(), "private-secret") {
		t.Fatalf("expected sanitized connection error, got %v", err)
	}
}

func TestImageTagListerNormalizesEmptyTagsAndRejectsRepeatedCursor(t *testing.T) {
	lister := &imageTagLister{client: &http.Client{Transport: registryRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		resp := registryResponse(http.StatusOK, `{"name":"team/app","tags":null}`)
		if req.URL.Query().Get("last") != "" {
			resp.Header.Set("Link", `<?last=v1>; rel="next"`)
		}
		return resp, nil
	})}}
	output, err := lister.ListTags(context.Background(), "ghcr.io/team/app", "", nil)
	if err != nil || output.Tags == nil || len(output.Tags) != 0 {
		t.Fatalf("ListTags() = %#v, %v", output, err)
	}
	if _, err := lister.ListTags(context.Background(), "ghcr.io/team/app", "v1", nil); err == nil {
		t.Fatal("expected repeated cursor to fail")
	}
}
