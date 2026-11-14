package infra

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/real-uangi/edge-pilot/internal/servicecatalog/domain"
	"github.com/real-uangi/edge-pilot/internal/shared/dto"
	"github.com/real-uangi/edge-pilot/internal/shared/image"

	"github.com/real-uangi/allingo/common/business"
)

var repositoryPathPattern = regexp.MustCompile(`^[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*(?:/[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*)*$`)

type imageTagLister struct {
	client *http.Client
}

func NewImageTagLister() domain.ImageTagLister {
	return &imageTagLister{client: &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (l *imageTagLister) ListTags(ctx context.Context, imageRepo, cursor string, credential *dto.ResolvedRegistryCredential) (*dto.ServiceImageTagsOutput, error) {
	endpoint, repository, err := registryTagsEndpoint(imageRepo)
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("n", "100")
	if cursor != "" {
		query.Set("last", cursor)
	}
	endpoint.RawQuery = query.Encode()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, business.NewBadRequest("镜像仓库地址格式错误")
	}
	resp, err := l.client.Do(req)
	if err != nil {
		return nil, registryRequestError(ctx)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		challenge := resp.Header.Get("WWW-Authenticate")
		resp.Body.Close()
		authorization, err := l.authorization(ctx, challenge, repository, credential)
		if err != nil {
			return nil, err
		}
		req = req.Clone(ctx)
		req.Header.Set("Authorization", authorization)
		resp, err = l.client.Do(req)
		if err != nil {
			return nil, registryRequestError(ctx)
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, registryStatusError(resp.StatusCode)
	}
	var payload struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&payload); err != nil {
		return nil, business.NewErrorWithCode("镜像仓库返回了无效的 Tag 列表", http.StatusBadGateway)
	}
	if payload.Name != repository {
		return nil, business.NewErrorWithCode("镜像仓库返回了不匹配的仓库名称", http.StatusBadGateway)
	}
	for _, tag := range payload.Tags {
		if !image.ValidTag(tag) {
			return nil, business.NewErrorWithCode("镜像仓库返回了无效的 Tag", http.StatusBadGateway)
		}
	}
	nextCursor, err := registryNextCursor(endpoint, resp.Header.Values("Link"))
	if err != nil {
		return nil, err
	}
	if nextCursor != "" && nextCursor == cursor {
		return nil, business.NewErrorWithCode("镜像仓库返回了无效的分页位置", http.StatusBadGateway)
	}
	if payload.Tags == nil {
		payload.Tags = []string{}
	}
	return &dto.ServiceImageTagsOutput{ImageRepo: imageRepo, Tags: payload.Tags, NextCursor: nextCursor}, nil
}

func registryTagsEndpoint(imageRepo string) (*url.URL, string, error) {
	repo := strings.TrimSpace(imageRepo)
	if strings.ContainsAny(repo, "@?#\\") || strings.Contains(repo, "://") {
		return nil, "", business.NewBadRequest("镜像仓库地址需为不含 Tag 的仓库名称")
	}
	first, rest, hasSlash := strings.Cut(repo, "/")
	host, repository := "docker.io", repo
	if hasSlash && (strings.ContainsAny(first, ".:") || first == "localhost") {
		host, repository = strings.ToLower(first), rest
	}
	if host == "docker.io" || host == "index.docker.io" || host == "registry-1.docker.io" {
		host = "registry-1.docker.io"
		if !strings.Contains(repository, "/") {
			repository = "library/" + repository
		}
	}
	if len(repository) > 255 || !repositoryPathPattern.MatchString(repository) {
		return nil, "", business.NewBadRequest("镜像仓库名称格式错误，请填写不含 Tag 的仓库名称")
	}
	endpoint, err := url.Parse("https://" + host + "/v2/" + repository + "/tags/list")
	if err != nil || endpoint.Hostname() == "" || endpoint.User != nil {
		return nil, "", business.NewBadRequest("镜像仓库地址格式错误")
	}
	return endpoint, repository, nil
}

func (l *imageTagLister) authorization(ctx context.Context, challenge, repository string, credential *dto.ResolvedRegistryCredential) (string, error) {
	scheme, rawParams, ok := strings.Cut(strings.TrimSpace(challenge), " ")
	if !ok {
		return "", business.NewBadRequest("镜像仓库未提供支持的认证方式")
	}
	var paramsBuilder strings.Builder
	quoted, escaped := false, false
	for _, char := range rawParams {
		if char == ',' && !quoted {
			char = ';'
		}
		paramsBuilder.WriteRune(char)
		if escaped {
			escaped = false
		} else if char == '\\' && quoted {
			escaped = true
		} else if char == '"' {
			quoted = !quoted
		}
	}
	_, params, err := mime.ParseMediaType("challenge; " + paramsBuilder.String())
	if err != nil {
		return "", business.NewErrorWithCode("镜像仓库返回了无效的认证信息", http.StatusBadGateway)
	}
	if strings.EqualFold(scheme, "Basic") {
		if credential == nil {
			return "", business.NewBadRequest("镜像仓库需要认证，请配置仓库凭据")
		}
		req := &http.Request{Header: make(http.Header)}
		req.SetBasicAuth(credential.Username, credential.Secret)
		return req.Header.Get("Authorization"), nil
	}
	if !strings.EqualFold(scheme, "Bearer") {
		return "", business.NewBadRequest("镜像仓库未提供支持的认证方式")
	}
	realm, err := url.Parse(params["realm"])
	if err != nil || realm.Scheme != "https" || realm.Hostname() == "" || realm.User != nil || realm.Fragment != "" {
		return "", business.NewErrorWithCode("镜像仓库返回了无效的认证地址", http.StatusBadGateway)
	}
	query := realm.Query()
	query.Set("scope", "repository:"+repository+":pull")
	if service := params["service"]; service != "" {
		query.Set("service", service)
	}
	realm.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, realm.String(), nil)
	if err != nil {
		return "", business.NewErrorWithCode("镜像仓库返回了无效的认证地址", http.StatusBadGateway)
	}
	if credential != nil {
		req.SetBasicAuth(credential.Username, credential.Secret)
	}
	resp, err := l.client.Do(req)
	if err != nil {
		return "", registryRequestError(ctx)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", registryStatusError(resp.StatusCode)
	}
	var token struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&token); err != nil {
		return "", business.NewErrorWithCode("镜像仓库返回了无效的认证令牌", http.StatusBadGateway)
	}
	if token.Token == "" {
		token.Token = token.AccessToken
	}
	if token.Token == "" {
		return "", business.NewErrorWithCode("镜像仓库未返回认证令牌", http.StatusBadGateway)
	}
	return "Bearer " + token.Token, nil
}

func registryNextCursor(endpoint *url.URL, links []string) (string, error) {
	for _, header := range links {
		for _, entry := range strings.Split(header, ",") {
			reference, attributes, ok := strings.Cut(strings.TrimSpace(entry), ">")
			if !ok || !strings.HasPrefix(reference, "<") {
				return "", business.NewErrorWithCode("镜像仓库返回了无效的分页信息", http.StatusBadGateway)
			}
			_, params, err := mime.ParseMediaType("link" + attributes)
			if err != nil {
				return "", business.NewErrorWithCode("镜像仓库返回了无效的分页信息", http.StatusBadGateway)
			}
			isNext := false
			for _, relation := range strings.Fields(params["rel"]) {
				if relation == "next" {
					isNext = true
				}
			}
			if !isNext {
				continue
			}
			next, err := endpoint.Parse(strings.TrimPrefix(reference, "<"))
			if err != nil || next.Scheme != endpoint.Scheme || !strings.EqualFold(next.Host, endpoint.Host) || next.Path != endpoint.Path || next.User != nil || next.Fragment != "" {
				return "", business.NewErrorWithCode("镜像仓库返回了跨仓库的分页地址", http.StatusBadGateway)
			}
			cursor := next.Query().Get("last")
			if !image.ValidTag(cursor) {
				return "", business.NewErrorWithCode("镜像仓库返回了无效的分页位置", http.StatusBadGateway)
			}
			return cursor, nil
		}
	}
	return "", nil
}

func registryRequestError(ctx context.Context) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return business.NewErrorWithCode("镜像仓库请求超时，请重试", http.StatusGatewayTimeout)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return business.NewErrorWithCode("无法连接镜像仓库，请检查仓库地址和网络", http.StatusBadGateway)
}

func registryStatusError(status int) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return business.NewBadRequest("镜像仓库认证失败或无访问权限，请检查仓库凭据")
	case http.StatusNotFound:
		return business.NewBadRequest("镜像仓库不存在或无访问权限")
	case http.StatusTooManyRequests:
		return business.NewErrorWithCode("镜像仓库请求过于频繁，请稍后重试", http.StatusTooManyRequests)
	default:
		return business.NewErrorWithCode("镜像仓库请求失败，请稍后重试", http.StatusBadGateway)
	}
}
