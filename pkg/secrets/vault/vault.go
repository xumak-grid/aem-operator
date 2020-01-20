package vault

import (
	"fmt"

	"github.com/hashicorp/vault/api"
	"github.com/xumak-grid/aem-operator/pkg/secrets"
)

type vaultSecretService struct {
	client *api.Client
}

// NewSecretService returns a new secret service implementation.
func NewSecretService() (secrets.SecretService, error) {
	// TODO: Get token and address from K8S
	// For testing VAULT_TOKEN, VAULT_ADDRESS should be in the environment.
	vaultClient, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return nil, err
	}
	vaultService := vaultSecretService{client: vaultClient}
	return &vaultService, nil
}

func (vss *vaultSecretService) Get(key string) (map[string]interface{}, error) {
	s, err := vss.client.Logical().Read(key)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return make(map[string]interface{}), nil
	}
	return s.Data, nil
}

func (vss *vaultSecretService) Put(key string, value map[string]interface{}) error {
	_, err := vss.client.Logical().Write(key, value)
	return err
}

func (vss *vaultSecretService) Delete(key string) error {
	_, err := vss.client.Logical().Delete(key)
	return err
}

// CleanUp deletes secrets under the especified path
// this is usually when the deployment is deleted
// example path secret/demo/dev
func (vss *vaultSecretService) CleanUp(path string) error {
	s, err := vss.client.Logical().List(path)
	if err != nil {
		return err
	}
	if s == nil {
		return nil
	}
	data, ok := s.Data["keys"].([]interface{})
	if !ok {
		return fmt.Errorf("nothing to clean in: %v", path)
	}
	for _, i := range data {
		k, _ := i.(string)
		// joins the path with the k to obtain the key
		keyDelete := fmt.Sprintf("%v/%v", path, k)
		err := vss.Delete(keyDelete)
		if err != nil {
			return err
		}
	}
	return nil
}
