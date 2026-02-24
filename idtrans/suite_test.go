package idtrans_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestIDTrans(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IDTrans Suite")
}
