package credhub_test

import (
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	credhub "github.com/cloudfoundry-community/go-credhub"
)

func TestGenerateCredentials(t *testing.T) {
	spec.Run(t, "GenerateCredentials", testGenerateCredentials, spec.Report(report.Terminal{}))
}

func testGenerateCredentials(t *testing.T, when spec.G, it spec.S) {
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

	when("Testing Generate Credential", func() {
		it("should generate a password credential", func() {
			params := make(map[string]interface{})
			params["length"] = 30
			cred, err := chClient.Generate("/example-generated", "password", params)
			Expect(err).To(Not(HaveOccurred()))
			Expect(cred.Type).To(Equal(credhub.Password))
			Expect(cred.Value).To(BeAssignableToTypeOf("expected"))
			Expect(cred.Value).To(HaveLen(30))
		})
	})

	when("Testing Regenerate Credential", func() {
		it("should regenerate a password credential", func() {
			cred, err := chClient.Regenerate("/example-password")
			Expect(err).To(Not(HaveOccurred()))
			Expect(cred.Type).To(Equal(credhub.Password))
			Expect(cred.Value).To(BeAssignableToTypeOf("expected"))
			Expect(cred.Value).To(BeEquivalentTo("P$<MNBVCXZ;lkjhgfdsa0987654321"))
		})
	})
}
