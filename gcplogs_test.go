package gcplogs

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultProjectID(t *testing.T) {
	// create a sandbox directory and sanitize the environment
	tempdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempdir)

	// save the state of special environment variables and remove them so the test works
	specialEnvVars := []string{ProjectEnvVar, "GOOGLE_APPLICATION_CREDENTIALS", "PATH"}
	origValues := map[string]string{}
	for _, key := range specialEnvVars {
		origValues[key] = os.Getenv(key)
		os.Unsetenv(key)
	}
	defer func() {
		for key, value := range origValues {
			os.Setenv(key, value)
		}
	}()
	os.Setenv("PATH", tempdir)

	projectID := DefaultProjectID()
	if projectID != "" {
		t.Fatal("Initial project ID must be empty; some environment must be wrong?")
	}

	// point this to application default credentials
	keyPath := filepath.Join(tempdir, "key.json")
	err = ioutil.WriteFile(keyPath, []byte(invalidServiceAccountKey), 0600)
	if err != nil {
		t.Fatal(err)
	}
	os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", keyPath)

	projectID = DefaultProjectID()
	if projectID != "bigquery-tools" {
		t.Error("project ID must be bigquery-tools with key:", projectID)
	}

	// override with GOOGLE_CLOUD_PROJECT: Used with Cloud Shell, new App Engine
	os.Setenv(ProjectEnvVar, "env-project")
	projectID = DefaultProjectID()
	if projectID != "env-project" {
		t.Error("Project environment variable must take priority:", projectID)
	}

	os.Unsetenv(ProjectEnvVar)
	os.Remove(keyPath)
	if DefaultProjectID() != "" {
		t.Error("need to reset environment to not find project ID")
	}

	// set up a fake gcloud
	gcloudPath := filepath.Join(tempdir, "gcloud")
	err = ioutil.WriteFile(gcloudPath, []byte(fakeGcloud), 0700)
	if err != nil {
		t.Fatal(err)
	}
	// fmt.Println("WTF", tempdir)
	// time.Sleep(time.Minute)
	projectID = DefaultProjectID()
	if projectID != "gcloud-project-id" {
		t.Error("incorrect gcloud project:", projectID)
	}
	args, err := ioutil.ReadFile(gcloudPath + ".args")
	if err != nil {
		t.Fatal(err)
	}
	const expectedArgs = "config get-value core/project\n"
	if string(args) != expectedArgs {
		t.Error("wrong gcloud args:", string(args))
	}
}

const fakeGcloud = `#!/bin/sh
echo $@ > $0.args
echo "gcloud-project-id"`

// This is a revoked service account.
const invalidServiceAccountKey = `{
  "type": "service_account",
  "project_id": "bigquery-tools",
  "private_key_id": "REDACTED",
  "private_key": "REDACTED",
  "client_email": "example@bigquery-tools.iam.gserviceaccount.com",
  "client_id": "103262392421472942815",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/example%40bigquery-tools.iam.gserviceaccount.com"
}`

func TestTracerFromRequest(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"invalid", ""},
		{"105445aa7843bc8bf206b120001000/0;o=1", "projects/test_id/traces/105445aa7843bc8bf206b120001000"},
	}

	tracer := &Tracer{"test_id"}
	zeroTracer := &Tracer{}

	for i, test := range tests {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(TraceHeader, test.input)

		output := tracer.FromRequest(req)
		if output != test.expected {
			t.Errorf("%d: FromRequest(%#v)=%#v; expected %#v", i, test.input, output, test.expected)
		}

		zeroOutput := zeroTracer.FromRequest(req)
		if zeroOutput != "" {
			t.Errorf("%d: FromRequest() must return the empty string if ProjectID is not set: %#v",
				i, zeroOutput)
		}
	}
}
