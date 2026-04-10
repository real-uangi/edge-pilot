package application

import (
	"edge-pilot/internal/registrycredential/domain"
	releasedomain "edge-pilot/internal/release/domain"
	"edge-pilot/internal/shared/dto"
	"edge-pilot/internal/shared/model"
	"strings"

	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/business"
)

type Service struct {
	repo   domain.Repository
	crypto *Crypto
}

func NewService(repo domain.Repository, crypto *Crypto) *Service {
	return &Service{
		repo:   repo,
		crypto: crypto,
	}
}

func (s *Service) Create(req dto.UpsertRegistryCredentialRequest) (*dto.RegistryCredentialOutput, error) {
	host, err := normalizeRegistryHost(req.RegistryHost)
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.GetByRegistryHost(host)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, business.NewErrorWithCode("registryHost 已存在", 409)
	}
	ciphertext, keyVersion, err := s.crypto.Encrypt(strings.TrimSpace(req.Secret))
	if err != nil {
		return nil, err
	}
	entity := &model.RegistryCredential{
		ID:               uuid.New(),
		RegistryHost:     host,
		Username:         strings.TrimSpace(req.Username),
		SecretCiphertext: ciphertext,
		SecretKeyVersion: keyVersion,
	}
	if entity.Username == "" {
		return nil, business.NewBadRequest("username 不能为空")
	}
	if strings.TrimSpace(req.Secret) == "" {
		return nil, business.NewBadRequest("secret 不能为空")
	}
	if err := s.repo.Create(entity); err != nil {
		return nil, err
	}
	output := toOutput(entity)
	return &output, nil
}

func (s *Service) Update(id uuid.UUID, req dto.UpsertRegistryCredentialRequest) (*dto.RegistryCredentialOutput, error) {
	current, err := s.repo.Get(id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, business.ErrNotFound
	}
	host, err := normalizeRegistryHost(req.RegistryHost)
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.GetByRegistryHost(host)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.ID != id {
		return nil, business.NewErrorWithCode("registryHost 已存在", 409)
	}
	if strings.TrimSpace(req.Username) == "" {
		return nil, business.NewBadRequest("username 不能为空")
	}
	if strings.TrimSpace(req.Secret) == "" {
		return nil, business.NewBadRequest("secret 不能为空")
	}
	ciphertext, keyVersion, err := s.crypto.Encrypt(strings.TrimSpace(req.Secret))
	if err != nil {
		return nil, err
	}
	current.RegistryHost = host
	current.Username = strings.TrimSpace(req.Username)
	current.SecretCiphertext = ciphertext
	current.SecretKeyVersion = keyVersion
	if err := s.repo.Update(current); err != nil {
		return nil, err
	}
	output := toOutput(current)
	return &output, nil
}

func (s *Service) Delete(id uuid.UUID) error {
	current, err := s.repo.Get(id)
	if err != nil {
		return err
	}
	if current == nil {
		return business.ErrNotFound
	}
	return s.repo.Delete(id)
}

func (s *Service) Get(id uuid.UUID) (*dto.RegistryCredentialOutput, error) {
	current, err := s.repo.Get(id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, business.ErrNotFound
	}
	output := toOutput(current)
	return &output, nil
}

func (s *Service) List() ([]dto.RegistryCredentialOutput, error) {
	items, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	output := make([]dto.RegistryCredentialOutput, 0, len(items))
	for i := range items {
		output = append(output, toOutput(&items[i]))
	}
	return output, nil
}

func (s *Service) ResolveForImageRepo(imageRepo string) (*releasedomain.ResolvedRegistryCredential, error) {
	host := ResolveRegistryHost(imageRepo)
	current, err := s.repo.GetByRegistryHost(host)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, nil
	}
	secret, err := s.crypto.Decrypt(current.SecretCiphertext, current.SecretKeyVersion)
	if err != nil {
		return nil, err
	}
	return &releasedomain.ResolvedRegistryCredential{
		Host:     current.RegistryHost,
		Username: current.Username,
		Secret:   secret,
	}, nil
}

func toOutput(entity *model.RegistryCredential) dto.RegistryCredentialOutput {
	return dto.RegistryCredentialOutput{
		ID:               entity.ID,
		RegistryHost:     entity.RegistryHost,
		Username:         entity.Username,
		SecretConfigured: strings.TrimSpace(entity.SecretCiphertext) != "",
		CreatedAt:        entity.CreatedAt,
		UpdatedAt:        entity.UpdatedAt,
	}
}

func normalizeRegistryHost(host string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(host))
	if value == "" {
		return "", business.NewBadRequest("registryHost 不能为空")
	}
	if strings.Contains(value, "://") || strings.Contains(value, "/") {
		return "", business.NewBadRequest("registryHost 必须是仓库 host")
	}
	return value, nil
}
