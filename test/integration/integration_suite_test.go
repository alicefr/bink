package integration_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bootc-dev/bink/test/integration/helpers"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bink Integration Suite")
}

var _ = BeforeSuite(func() {
	GinkgoWriter.Println("=== Integration Test Suite Setup ===")

	// Verify podman is available
	helpers.RequireCommand("podman")

	// Verify bink binary exists
	helpers.RequireBink()

	// Verify test images exist
	helpers.RequireImage("localhost/cluster:latest")
	helpers.RequireImage("localhost/fedora-bootc-k8s-image:latest")

	GinkgoWriter.Println("✓ All prerequisites verified")
})

var _ = AfterSuite(func() {
	GinkgoWriter.Println("=== Integration Test Suite Cleanup ===")

	// Cleanup any leftover test clusters
	helpers.CleanupAllTestClusters()

	GinkgoWriter.Println("✓ Cleanup complete")
})
