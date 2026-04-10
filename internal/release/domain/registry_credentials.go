package domain

type ResolvedRegistryCredential struct {
	Host     string
	Username string
	Secret   string
}

type RegistryCredentialResolver interface {
	ResolveForImageRepo(imageRepo string) (*ResolvedRegistryCredential, error)
}
