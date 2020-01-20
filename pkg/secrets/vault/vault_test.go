package vault

import (
	"reflect"
	"testing"
)

func TestVaultGet(t *testing.T) {
	ss, err := NewSecretService()
	if err != nil {
		t.Fatal("error", err)
	}
	data := map[string]interface{}{
		"Test": "TestString",
	}
	ss.Put("secret/test", data)
	s, err := ss.Get("secret/test")
	if err != nil {
		t.Fatal("error", err)
	}

	if !reflect.DeepEqual(data, s) {
		t.Fatal("Data is not equal")
	}

	err = ss.Delete("secret/test")
	if err != nil {
		t.Fatal("error", err)
	}
}

func TestVaultGetCleanUp(t *testing.T) {
	ss, err := NewSecretService()
	if err != nil {
		t.Fatal("error", err)
	}
	data := map[string]interface{}{
		"Test": "TestString",
	}
	ss.Put("secret/test/env/secret1", data)
	ss.Put("secret/test/env/secret2", data)

	err = ss.CleanUp("secret/test/env")
	if err != nil {
		t.Fatal("error", err)
	}
	secret, err := ss.Get("secret/test/env/secret1")
	if err != nil {
		t.Fatal("error", err)
	}
	secret2, err := ss.Get("secret/test/env/secret2")
	if err != nil {
		t.Fatal("error", err)
	}
	if len(secret) > 0 || len(secret2) > 0 {
		t.Error("secrets should not contain values", secret)
	}
}
