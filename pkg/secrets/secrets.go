package secrets

// SecretService should implement secret storage management.
type SecretService interface {
	Get(key string) (map[string]interface{}, error)
	Put(key string, value map[string]interface{}) error
	Delete(Key string) error
	CleanUp(path string) error
}
