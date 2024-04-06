package main

import (
	"encoding/base64"
	"errors"
	"os"
	"testing"
	"text/template"

	acmetest "github.com/cert-manager/cert-manager/test/acme"
)

var (
	testZoneName = os.Getenv("TEST_ZONE_NAME")
	manifestPath = "testdata/infomaniak"
)

func createSecretFile() error {
	apiToken := os.Getenv("INFOMANIAK_TOKEN")
	if apiToken == "" {
		return errors.New("INFOMANIAK_TOKEN should be defined")
	}
	apiTokenBase64 := base64.StdEncoding.EncodeToString([]byte(apiToken))

	secretTmpl := `---
apiVersion: v1
kind: Secret
metadata:
  name: infomaniak-api-credentials
type: Opaque
data:
  api-token: {{.}}
`
	secretFile, err := os.Create(manifestPath + "/api-key.yaml")
	if err != nil {
		return err
	}
	defer secretFile.Close()

	tmpl, err := template.New("api-key.yaml").Parse(secretTmpl)
	if err != nil {
		return err
	}
	err = tmpl.Execute(secretFile, apiTokenBase64)

	return nil
}

func createConfig() error {
	config := []byte(`{
	"apiTokenSecretRef": {
		"name": "infomaniak-api-credentials",
		"key": "api-token"
	}
}
`)
	err := os.WriteFile(manifestPath+"/config.json", config, 0644)
	if err != nil {
		return err
	}

	return nil
}

func runTestSuite(t *testing.T, zone string) {
	// The manifest path should contain a file named config.json that is a
	// snippet of valid configuration that should be included on the
	// ChallengeRequest passed as part of the test cases.

	if len(zone) == 0 || zone == "api." {
		t.Fatal("Can't run tests on empty zone, please define TEST_ZONE_NAME")
	}

	// Create the secret file from INFOMANIAK_TOKEN env. variable
	if err := createSecretFile(); err != nil {
		t.Fatal(err)
	}

	// Create the config file from TEST_METHOD env. variable
	if err := createConfig(); err != nil {
		t.Fatal(err)
	}

	fixture := acmetest.NewFixture(&infomaniakDNSProviderSolver{},
		acmetest.SetResolvedZone(zone),
		acmetest.SetAllowAmbientCredentials(false),
		acmetest.SetManifestPath(manifestPath),
	)

	fixture.RunConformance(t)

}

func TestRunsSuiteIKAPI(t *testing.T) {
	runTestSuite(t, testZoneName)
}

func TestRunsSuiteIKAPISubdomain(t *testing.T) {
	runTestSuite(t, "api."+testZoneName)
}
