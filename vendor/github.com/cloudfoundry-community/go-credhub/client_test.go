package credhub_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	credhub "github.com/cloudfoundry-community/go-credhub"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

type authRoundTripper struct {
	orig http.RoundTripper
}

func (a *authRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Add("authorization", "bearer abcd")
	return a.orig.RoundTrip(r)
}

func getAuthenticatedClient(hc *http.Client) *http.Client {
	tr := &authRoundTripper{
		orig: hc.Transport,
	}

	hc.Transport = tr
	return hc
}

func TestInvalidValueTypeConversion(t *testing.T) {
	spec.Run(t, "InvalidValueTypeConversion", testInvalidValueTypeConversion, spec.Report(report.Terminal{}))
}

func testInvalidValueTypeConversion(t *testing.T, when spec.G, it spec.S) {
	var (
		server   *httptest.Server
		chClient *credhub.Client
	)

	it.Before(func() {
		RegisterTestingT(t)
		server = mockCredhubServer()
		chClient = credhub.New(server.URL, getAuthenticatedClient(server.Client()))
	})

	it.After(func() {
		server.Close()
	})

	when("converting to the wrong value types", func() {
		it("fails", func() {
			var (
				cred *credhub.Credential
				err  error
			)

			cred, err = chClient.GetLatestByName("/concourse/common/sample-rsa")
			Expect(err).NotTo(HaveOccurred())
			_, err = credhub.SSHValue(*cred)
			Expect(err).To(HaveOccurred())

			cred, err = chClient.GetLatestByName("/concourse/common/sample-ssh")
			Expect(err).NotTo(HaveOccurred())
			_, err = credhub.UserValue(*cred)
			Expect(err).To(HaveOccurred())

			cred, err = chClient.GetLatestByName("/concourse/common/sample-user")
			Expect(err).NotTo(HaveOccurred())
			_, err = credhub.CertificateValue(*cred)
			Expect(err).To(HaveOccurred())

			cred, err = chClient.GetLatestByName("/concourse/common/sample-certificate")
			Expect(err).NotTo(HaveOccurred())
			_, err = credhub.RSAValue(*cred)
			Expect(err).To(HaveOccurred())
		})
	})
}

func vcapServicesDeepEnoughEquals(a, b string) bool {
	var err error

	actual := new(map[string][]map[string]interface{})
	expected := new(map[string][]map[string]interface{})

	if err = json.Unmarshal([]byte(a), actual); err != nil {
		return false
	}

	if err = json.Unmarshal([]byte(b), expected); err != nil {
		return false
	}

	if err = normalizeCredentials(actual); err != nil {
		return false
	}

	if err = normalizeCredentials(expected); err != nil {
		return false
	}

	matcher := &BeEquivalentToMatcher{
		Expected: *expected,
	}

	equal, err := matcher.Match(*actual)
	return equal && err == nil
}

func normalizeCredentials(vcap *map[string][]map[string]interface{}) error {
	for serviceType := range *vcap {
		for i := range (*vcap)[serviceType] {
			if _, ok := (*vcap)[serviceType][i]["credentials"]; ok {
				(*vcap)[serviceType][i]["credentials"] = "TEST-NORMALIZATION"
			}
		}
	}

	return nil
}
